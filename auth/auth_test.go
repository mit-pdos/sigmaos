package auth_test

import (
	"path"
	"testing"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keyclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	REALM1 sp.Trealm = "testrealm1"
)

func TestCompile(t *testing.T) {
}

func TestStartStop(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	db.DPrintf(db.TEST, "Started successfully")
	rootts.Shutdown()
}

func TestStartMultiNodeStop(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	err := rootts.BootNode(1)
	assert.Nil(rootts.T, err, "Err boot node: %v", err)
	db.DPrintf(db.TEST, "Started successfully")
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

	// Create a new sigma clnt
	pe := proc.NewAddedProcEnv(rootts.ProcEnv())
	pe.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID("scoped-down-principal"),
		pe.GetRealm(),
	))
	// Clear AWS secrets
	pe.SetSecrets(map[string]*sp.SecretProto{})

	sc1, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err, "Err NewClnt: %v", err)

	sts, err = sc1.GetDir(sp.NAMED)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "realm names %v", sp.Names(sts))

	sts, err = sc1.GetDir(path.Join(sp.S3, "~local"))
	assert.NotNil(t, err)
	db.DPrintf(db.TEST, "s3 contents %v", sp.Names(sts))

	sts, err = sc1.GetDir(path.Join(sp.S3, "~local", "9ps3"))
	assert.NotNil(t, err, "Successfully got dir. \n\tPE: %v", sc1.ProcEnv())
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

// Test that an unauthorized principal can't write a public key to keyd (or
// overwrite an existing one)
func TestMaliciousPrincipalKeydFail(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	kc1 := keyclnt.NewKeyClnt[*jwt.SigningMethodECDSA](rootts.SigmaClnt)
	// Make sure the root sigma clnt can get a key
	k, err := kc1.GetKey(jwt.SigningMethodES256, auth.SIGMA_DEPLOYMENT_MASTER_SIGNER)
	assert.Nil(t, err)
	// Make sure the root sigma clnt can set an existing key
	err = kc1.SetKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, k)
	assert.Nil(t, err)
	// Make sure the root sigma clnt can set a new key
	err = kc1.SetKey(sp.Tsigner("woohoo"), k)
	assert.Nil(t, err)

	// Create a new sigma clnt
	pe := proc.NewAddedProcEnv(rootts.ProcEnv())
	pe.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID("scoped-down-principal"),
		pe.GetRealm(),
	))
	// Create a new, more restricted sigmaclnt
	sc1, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err, "Err NewClnt: %v", err)

	kc2 := keyclnt.NewKeyClnt[*jwt.SigningMethodECDSA](sc1)
	// Make sure the new sigma clnt can get a key
	k2, err := kc2.GetKey(jwt.SigningMethodES256, auth.SIGMA_DEPLOYMENT_MASTER_SIGNER)
	assert.Nil(t, err)
	// Make sure the new sigma clnt cannot set an existing key
	err = kc2.SetKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, k2)
	assert.NotNil(t, err)
	// Make sure the root sigma clnt cannot set a new key
	err = kc1.SetKey(sp.Tsigner("woohaa"), k2)
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

func TestAWSRestrictedProfileS3BucketAccess(t *testing.T) {
	// First, try to get restricted AWS secrets
	s3secrets, err1 := auth.GetAWSSecrets(sp.AWS_S3_RESTRICTED_PROFILE)
	if !assert.Nil(t, err1, "Can't get secrets for aws profile %v: %v", sp.AWS_S3_RESTRICTED_PROFILE, err1) {
		return
	}

	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn1 := path.Join(sp.S3, "~local", "mr-restricted")
	pn2 := path.Join(pn1, "gutenberg")
	sts, err := rootts.GetDir(pn1)
	assert.Nil(t, err)
	sts, err = rootts.GetDir(pn2)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "s3 contents %v", sp.Names(sts))

	// Create a new sigma clnt
	pe := proc.NewAddedProcEnv(rootts.ProcEnv())
	pe.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID("scoped-down-principal"),
		pe.GetRealm(),
	))

	// Load scoped-down AWS secrets
	pe.SetSecrets(map[string]*sp.SecretProto{"s3": s3secrets})

	sc1, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err, "Err NewClnt: %v", err)

	sts2, err := sc1.GetDir(path.Join(sp.S3, "~local") + "/")
	assert.Nil(t, err, "Err GetDir [%v]: %v", path.Join(sp.S3, "~local/"), err)
	db.DPrintf(db.TEST, "accessbile s3 buckets %v", sp.Names(sts2))

	sts2, err = sc1.GetDir(path.Join(sp.S3, "~local", "9ps3"))
	assert.NotNil(t, err, "Successfully got dir. \n\tPE: %v", sc1.ProcEnv())

	sts2, err = sc1.GetDir(pn1)
	assert.Nil(t, err)
	sts2, err = sc1.GetDir(pn2)
	assert.Nil(t, err)
	assert.True(t, len(sts2) == 8, "Wrong number of gutenberg entries: %v != 8", len(sts2))
	db.DPrintf(db.TEST, "s3 contents (using restricted AWS account/role) %v", sp.Names(sts2))

	rootts.Shutdown()
}
