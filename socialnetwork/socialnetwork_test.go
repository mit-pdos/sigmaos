package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	"sigmaos/rand"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	"sigmaos/socialnetwork/proto"
	"sigmaos/protdevclnt"
	"github.com/stretchr/testify/assert"
	"time"
)

type TstateSN struct {
	*test.Tstate
	jobname string
	snCfg   *sn.SocialNetworkConfig
}

func makeTstateSN(t *testing.T, srvs []sn.Srv) *TstateSN {
	var err error
	tssn := &TstateSN{}
	tssn.jobname = rand.String(8)
	tssn.Tstate = test.MakeTstateAll(t)
	assert.Nil(tssn.T, err, "test kernel should start properly.")
	tssn.snCfg, err = sn.MakeConfig(tssn.SigmaClnt, tssn.jobname, srvs)
	assert.Nil(tssn.T, err, "config should be created properly.")
	return tssn
}

func TestFindMeanlingServer(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-mol", test.Overlays, 1}})
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
	stopErr := snCfg.Stop()
	assert.Nil(t, stopErr, "Procs should stop properly")
	tssn.Shutdown()
}

func TestUser(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-user", test.Overlays, 1}})
	snCfg := tssn.snCfg

	// create a RPC client and query
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_USER)
	assert.Nil(t, err, "RPC client should be created properly")

	// check user
	arg_check := proto.CheckUserRequest{Username: "test_user"}
	res_check := proto.CheckUserResponse{}
	err = pdc.RPC("User.CheckUser", &arg_check, &res_check)
	assert.Nil(t, err)
	assert.Equal(t, "No", res_check.Ok)

	// register user
	arg_reg := proto.RegisterUserRequest{
		Firstname: "Alice", Lastname: "Test", Username: "user_0", Password: "xxyyzz"}
	res_reg := proto.RegisterUserResponse{}
	err = pdc.RPC("User.RegisterUser", &arg_reg, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "Username user_0 already exist", res_reg.Ok)

	arg_reg.Username = "test_user"
	err = pdc.RPC("User.RegisterUser", &arg_reg, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_reg.Ok)

	err = pdc.RPC("User.CheckUser", &arg_check, &res_check)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_check.Ok)


    // new user login

	//stop server
	time.Sleep(2 * time.Second)
	stopErr := snCfg.Stop()
	assert.Nil(t, stopErr, "Procs should stop properly")
	tssn.Shutdown()

}
