package socialnetwork_test

import (
	"testing"
	"sigmaos/socialnetwork"
	"sigmaos/socialnetwork/proto"
	"sigmaos/protdevclnt"
	"github.com/stretchr/testify/assert"
	"time"
)

func TestFindMeaningLocal(t *testing.T) {
	mol := socialnetwork.MeaningOfLife{}
	arg := proto.MoLRequest{
		Name: "test",
	} 
	res := proto.MoLResult{}
	mol.FindMeaning(nil, arg, &res)
	assert.Equal(t, float32(42), res.Meaning)
}

func TestFindMeanlingServer(t *testing.T) {
	// start server
	snCfg, startErr := socialnetwork.MakeDefaultSocialNetworkConfig()
	assert.Nil(t, startErr, "config should be created properly")

	// create a RPC client and query
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, socialnetwork.MOL_SERVICE_NAME)
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
	shutdownErr := snCfg.Shutdown()
	assert.Nil(t, shutdownErr, "Kernel should shut down properly")

}
