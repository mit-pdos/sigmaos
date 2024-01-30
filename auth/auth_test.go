package auth_test

import (
	"path"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	REALM1 sp.Trealm = "testrealm1"
)

func TestSignHMACToken(t *testing.T) {
	// TODO: generate key properly
	var hmacSecret []byte = []byte("PDOS")
	as, err := auth.NewHMACAuthSrv(proc.NOT_SET, hmacSecret)
	assert.Nil(t, err, "Err make auth clnt: %v", err)
	// Create the Claims
	claims := &auth.ProcClaims{
		PID:          "my-pid",
		AllowedPaths: []string{"/*"},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: 15000, // TODO: how to set these properly?
			Issuer:    "test",
		},
	}
	signedToken, err := as.NewToken(claims)
	assert.Nil(t, err, "Err sign token: %v", err)
	db.DPrintf(db.TEST, "Signed token: %v", signedToken)
}

func TestVerifyHMACToken(t *testing.T) {
	// TODO: generate key properly
	var hmacSecret []byte = []byte("PDOS")
	as, err := auth.NewHMACAuthSrv(proc.NOT_SET, hmacSecret)
	assert.Nil(t, err, "Err make auth clnt: %v", err)
	// Create the Claims
	claims := &auth.ProcClaims{
		PID:          "my-pid",
		AllowedPaths: []string{"/*"},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Minute * 1).Unix(),
			Issuer:    "test",
		},
	}
	signedToken, err := as.NewToken(claims)
	assert.Nil(t, err, "Err sign token: %v", err)
	db.DPrintf(db.TEST, "Signed token: %v", signedToken)
	claims2, err := as.VerifyTokenGetClaims(signedToken)
	assert.Nil(t, err, "Err verify token get claims: %v", err)
	db.DPrintf(db.TEST, "Signed token: %v", claims2)
}

func TestStartStop(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	db.DPrintf(db.TEST, "Started successfully")
	rootts.Shutdown()
}

func TestInspectNamespaceOK(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := rootts.GetDir(sp.NAMED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm named root %v", sp.Names(sts))

	assert.True(t, fslib.Present(sts, []string{sp.UXREL}), "initfs")

	sts, err = rootts.GetDir(path.Join(sp.S3, "~local", "9ps3"))
	assert.Nil(t, err, "Error getdir: %v", err)

	db.DPrintf(db.TEST, "9ps3 root %v", sp.Names(sts))

	sts, err = rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v", sp.Names(sts))

	sts2, err := rootts.GetDir(path.Join(sp.SCHEDD, sts[0].Name) + "/")
	assert.Nil(t, err, "Err getdir: %v", err)

	db.DPrintf(db.TEST, "sched contents %v", sp.Names(sts2))

	rootts.Shutdown()
}

// Test that a principal without a signed token can't access anything
func TestMaliciousPrincipalFail(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	// Create a new sigma clnt, with an unexpected principal
	pe := proc.NewAddedProcEnv(rootts.ProcEnv(), 1)
	pe.SetPrincipal(&sp.Tprincipal{
		ID:       "malicious-user",
		TokenStr: proc.NOT_SET,
	})
	sc1, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err, "Err NewClnt: %v", err)

	_, err = sc1.GetDir(sp.NAMED)
	assert.NotNil(t, err)

	sts, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v", sp.Names(sts))

	_, err = sc1.GetDir(path.Join(sp.SCHEDD, sts[0].Name) + "/")
	assert.NotNil(t, err)

	rootts.Shutdown()
}

// Test that a principal without a signed token can't access anything
func TestMaliciousPrincipalS3Fail(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := rootts.GetDir(path.Join(sp.S3, "~local", "9ps3"))
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "s3 contents %v", sp.Names(sts))

	// Create an auth server
	as, err := auth.NewHMACAuthSrv(proc.NOT_SET, []byte("PDOS"))
	assert.Nil(t, err)
	// Create a new sigma clnt
	pe := proc.NewAddedProcEnv(rootts.ProcEnv(), 1)
	// Clear AWS secrets
	pe.SetSecrets(map[string]*proc.ProcSecretProto{})
	pc := auth.NewProcClaims(pe)
	token, err := as.NewToken(pc)
	assert.Nil(t, err)
	// Set the token of the proc env to the newly authorized token
	pe.SetToken(token)

	sc1, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err, "Err NewClnt: %v", err)

	sts, err = sc1.GetDir(sp.NAMED)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "realm names %v", sp.Names(sts))

	sts, err = sc1.GetDir(path.Join(sp.S3, "~local"))
	assert.NotNil(t, err)
	db.DPrintf(db.TEST, "s3 contents %v", sp.Names(sts))

	sts, err = sc1.GetDir(path.Join(sp.S3, "~local", "9ps3"))
	assert.NotNil(t, err)
	db.DPrintf(db.TEST, "s3 contents %v", sp.Names(sts))

	sts, err = rootts.GetDir(path.Join(sp.S3, "~local", "9ps3"))
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "s3 contents %v", sp.Names(sts))

	sts, err = rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v", sp.Names(sts))

	_, err = sc1.GetDir(path.Join(sp.SCHEDD, sts[0].Name) + "/")
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestDelegateFullAccessOK(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	// Create a child proc, which should be able to access everything the test
	// program can access
	p1 := proc.NewProc("dirreader", []string{path.Join(sp.SCHEDD, "~any")})

	err := rootts.Spawn(p1)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Spawned proc")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := rootts.WaitExit(p1.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")

	// Make sure that WaitExit didn't return an error
	assert.Nil(t, err, "WaitExit error: %v", err)
	// Ensure the proc succeeded
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status not OK: %v", status)

	db.DPrintf(db.TEST, "Authorized child proc return status: %v", status)

	rootts.Shutdown()
}

func TestDelegateNoAccessFail(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p1 := proc.NewProc("dirreader", []string{path.Join(sp.UX, "~any")})
	// Wipe the list of allowed paths (except for schedd)
	p1.SetAllowedPaths([]string{sp.NAMED, path.Join(sp.SCHEDD, "*")})

	err := rootts.Spawn(p1)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Spawned proc")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := rootts.WaitExit(p1.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")

	// Make sure that WaitExit didn't return an error
	assert.Nil(t, err, "WaitExit error: %v", err)
	// Ensure the proc crashed
	assert.True(t, status != nil && status.IsStatusErr(), "Exit status not error: %v", status)

	db.DPrintf(db.TEST, "Unauthorized child proc return status: %v", status)

	rootts.Shutdown()
}

func TestDelegatePartialAccess(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p1 := proc.NewProc("dirreader", []string{path.Join(sp.S3, "~any")})
	// Only allow access to S3
	p1.SetAllowedPaths([]string{sp.NAMED, path.Join(sp.SCHEDD, "*")})

	err := rootts.Spawn(p1)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Spawned proc")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := rootts.WaitExit(p1.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")

	// Make sure that WaitExit didn't return an error
	assert.Nil(t, err, "WaitExit error: %v", err)
	// Ensure the proc crashed
	assert.True(t, status != nil && status.IsStatusErr(), "Exit status not error: %v", status)
	db.DPrintf(db.TEST, "Unauthorized child proc return status: %v", status)

	p2 := proc.NewProc("dirreader", []string{path.Join(sp.UX, "~any")})
	// Only allow access to UX
	p2.SetAllowedPaths([]string{sp.NAMED, path.Join(sp.SCHEDD, "*"), path.Join(sp.UX, "*")})

	err = rootts.Spawn(p2)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Spawned proc")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err = rootts.WaitExit(p2.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")

	// Make sure that WaitExit didn't return an error
	assert.Nil(t, err, "WaitExit error: %v", err)
	// Ensure the proc succeeded
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status not OK: %v", status)

	db.DPrintf(db.TEST, "Authorized child proc return status: %v", status)

	rootts.Shutdown()
}

func TestTryDelegateNonSubsetToChildFail(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	// Create an auth server
	as, err := auth.NewHMACAuthSrv(proc.NOT_SET, []byte("PDOS"))
	assert.Nil(t, err)

	// Create a new proc env to create a new client
	pe := proc.NewAddedProcEnv(rootts.ProcEnv(), 1)
	// Only let it talk to schedd and named
	pe.SetAllowedPaths([]string{sp.NAMED, path.Join(sp.SCHEDD, "*"), path.Join(sp.PROCQ, "*")})
	pc := auth.NewProcClaims(pe)
	token, err := as.NewToken(pc)
	assert.Nil(t, err)
	// Set the token of the proc env to the newly authorized token
	pe.SetToken(token)

	// Create a new client with the proc env
	sc1, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err, "Err NewClnt: %v", err)

	// Spawn a proc which can read a subset of the parent's allowed paths
	p1 := proc.NewProc("dirreader", []string{path.Join(sp.SCHEDD, "~any")})
	// Wipe the list of allowed paths (except for schedd)
	p1.SetAllowedPaths([]string{sp.NAMED, path.Join(sp.SCHEDD, "*")})
	err = sc1.Spawn(p1)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Spawned proc")
	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := sc1.WaitExit(p1.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	// Make sure that WaitExit didn't return an error
	assert.Nil(t, err, "WaitExit error: %v", err)
	// Ensure the proc succeeded
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status not OK: %v", status)
	db.DPrintf(db.TEST, "Authorized child proc return status: %v", status)

	// Spawn a proc which tries to access a superset of the parent's paths
	p2 := proc.NewProc("dirreader", []string{path.Join(sp.UX, "~any")})
	// Only allow access to UX
	p2.SetAllowedPaths([]string{sp.NAMED, path.Join(sp.SCHEDD, "*"), path.Join(sp.UX, "*")})
	err = sc1.Spawn(p2)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Spawned proc")
	db.DPrintf(db.TEST, "Pre waitexit")
	status, err = sc1.WaitExit(p2.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	// Make sure that WaitExit didn't return an error
	assert.Nil(t, err, "WaitExit error: %v", err)
	// Ensure the proc crashed
	assert.True(t, status != nil && status.IsStatusErr(), "Exit status not error: %v", status)
	db.DPrintf(db.TEST, "Unauthorized child proc return status: %v", status)
	rootts.Shutdown()
}
