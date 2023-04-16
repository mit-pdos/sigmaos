package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	"sigmaos/rand"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/socialnetwork/proto"
	"sigmaos/protdevclnt"
	"sigmaos/linuxsched"
	"github.com/stretchr/testify/assert"
	"time"
)

const (
	NSHARD = 4
)

type TstateSN struct {
	*test.Tstate
	jobname string
	snCfg *sn.SocialNetworkConfig
	dbu *sn.DBUtil
}

func makeTstateSN(t *testing.T, srvs []sn.Srv, nshard int) *TstateSN {
	var err error
	tssn := &TstateSN{}
	tssn.jobname = rand.String(8)
	tssn.Tstate = test.MakeTstateAll(t)
	nMoreKernel := (len(srvs)*2+NSHARD*2) / int(linuxsched.NCores)
	if nMoreKernel > 0 {
		dbg.DPrintf(dbg.ALWAYS, "%v / %v = %v more kernels are needed", 
			len(srvs)*2+NSHARD*2, linuxsched.NCores, nMoreKernel)	
		err = tssn.BootNode(nMoreKernel)
		assert.Nil(tssn.T, err)
	}
	tssn.snCfg, err = sn.MakeConfig(tssn.SigmaClnt, tssn.jobname, srvs, nshard, test.Overlays)
	assert.Nil(tssn.T, err, "config should initialize properly.")
	tssn.dbu, err = sn.MakeDBUtil(tssn.SigmaClnt)
	assert.Nil(tssn.T, err, "DBUtil should initialize properly.")
	return tssn
}

func (tssn *TstateSN) Shutdown() error {
	if stopErr := tssn.snCfg.Stop(); stopErr != nil {
		return stopErr
	}
	return tssn.Tstate.Shutdown()
}

func TestToyMeaningOfLife(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-mol", test.Overlays, 1}}, 0)
	snCfg := tssn.snCfg

	// create a RPC client and query
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_MOL)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := proto.MoLRequest{
		Name: "test",
	}
	res := proto.MoLResult{}
	err = pdc.RPC("MeaningOfLife.FindMeaning", &arg, &res)
	assert.Nil(t, err, "RPC call should succeed")
	assert.Equal(t, float32(42), res.Meaning)

	// sleep a while to print heartbeats then stop
	time.Sleep(2 * time.Second)
	assert.Nil(t, tssn.Shutdown())
}

func TestUser(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-user", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create a RPC client and query
	tssn.dbu.InitUser()
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_USER)
	assert.Nil(t, err, "RPC client should be created properly")

	// check user
	arg_check := proto.CheckUserRequest{Username: "test_user"}
	res_check := proto.UserResponse{}
	err = pdc.RPC("User.CheckUser", &arg_check, &res_check)
	assert.Nil(t, err)
	assert.Equal(t, "No", res_check.Ok)

	// register user
	arg_reg := proto.RegisterUserRequest{
		Firstname: "Alice", Lastname: "Test", Username: "user_0", Password: "xxyyzz"}
	res_reg := proto.UserResponse{}
	err = pdc.RPC("User.RegisterUser", &arg_reg, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "Username user_0 already exist", res_reg.Ok)

	arg_reg.Username = "test_user"
	err = pdc.RPC("User.RegisterUser", &arg_reg, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_reg.Ok)
	created_userid := res_reg.Userid

	// check user
	err = pdc.RPC("User.CheckUser", &arg_check, &res_check)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_check.Ok)
	assert.Equal(t, created_userid, res_check.Userid)

    // new user login
	arg_login := proto.LoginRequest{Username: "test_user", Password: "xxyy"}
	res_login := proto.UserResponse{}
	err = pdc.RPC("User.Login", &arg_login, &res_login)
	assert.Nil(t, err)
	assert.Equal(t, "Login Failure.", res_login.Ok)

	arg_login.Password = "xxyyzz"
	err = pdc.RPC("User.Login", &arg_login, &res_login)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_login.Ok)

	// verify cache contents
	user := &sn.User{}
	err = snCfg.CacheClnt.Get("user_by_uname_test_user", user)
	assert.Nil(t, err)
	assert.Equal(t, "Alice", user.Firstname)
	assert.Equal(t, "Test", user.Lastname)
	assert.Equal(t, created_userid, user.Userid)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func TestGraph(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-graph", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create a RPC client and query
	_, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_GRAPH)
	assert.Nil(t, err)
	
	//stop server
	assert.Nil(t, tssn.Shutdown())
}
