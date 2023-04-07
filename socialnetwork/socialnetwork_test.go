package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	"sigmaos/rand"
	sn "sigmaos/socialnetwork"
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

func TestFindMeaningLocal(t *testing.T) {
	mol := sn.MeaningOfLife{}
	arg := proto.MoLRequest{
		Name: "test",
	} 
	res := proto.MoLResult{}
	mol.FindMeaning(nil, arg, &res)
	assert.Equal(t, float32(42), res.Meaning)
}

func TestFindMeanlingServer(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-mol", test.Overlays, 1}})
	snCfg := tssn.snCfg

	// create a RPC client and query
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sn.MOL_SERVICE_NAME)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := proto.MoLRequest{
		Name: "test",
	}
	res := proto.MoLResult{}
	err = pdc.RPC("MeaningOfLife.FindMeaning", &arg, &res)
	assert.Nil(t, err, "RPC call should succeed")
	assert.Equal(t, float32(42), res.Meaning)

	// sleep a while to print heartbeats then stop
	time.Sleep(10 * time.Second)
	stopErr := snCfg.Stop()
	assert.Nil(t, stopErr, "Procs should stop properly")
	tssn.Shutdown()
}
