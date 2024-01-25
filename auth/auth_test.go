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

//func TestNoDelegationPrincipalFail(t *testing.T) {
//	rootts, err1 := test.NewTstateWithRealms(t)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//
//	p1 := proc.NewProc("sleeper", []string{"2s", "name/"})
//	// Wipe the token from the child proc's env
//	p1.GetProcEnv().Principal.TokenPresent = false
//
//	err := rootts.Spawn(p1)
//	assert.Nil(t, err, "Spawn")
//	db.DPrintf(db.TEST, "Spawned proc")
//
//	db.DPrintf(db.TEST, "Pre waitexit")
//	status, err := rootts.WaitExit(p1.GetPid())
//	db.DPrintf(db.TEST, "Post waitexit")
//
//	// Make sure that WaitExit didn't return an error
//	assert.Nil(t, err, "WaitExit error: %v", err)
//	// Ensure the proc crashed
//	assert.True(t, status != nil && status.IsStatusErr(), "Exit status not error: %v", status)
//
//	db.DPrintf(db.TEST, "Unauthorized child proc return status: %v", status)
//
//	rootts.Shutdown()
//}

func TestSignHMACToken(t *testing.T) {
	// TODO: generate key properly
	var hmacSecret []byte = []byte("PDOS")
	as, err := auth.NewHMACAuthSrv(hmacSecret)
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
	as, err := auth.NewHMACAuthSrv(hmacSecret)
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
