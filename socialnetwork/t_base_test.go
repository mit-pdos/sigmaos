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
	"sigmaos/fslib"
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
	nMoreKernel := ((len(srvs)*2 + NSHARD*2) - 1)  / int(linuxsched.NCores)
	if nMoreKernel > 0 {
		dbg.DPrintf(dbg.ALWAYS, "(%v * %v - 1) / %v = %v more kernels are needed", 
			len(srvs)*2 + NSHARD*2, sn.N_RPC_SESSIONS, linuxsched.NCores, nMoreKernel)	
		err = tssn.BootNode(nMoreKernel)
		assert.Nil(tssn.T, err)
	}
	tssn.snCfg, err = sn.MakeConfig(tssn.SigmaClnt, tssn.jobname, srvs, nshard, true, test.Overlays)
	assert.Nil(tssn.T, err, "config should initialize properly.")
	tssn.dbu, err = sn.MakeDBUtil(tssn.SigmaClnt)
	assert.Nil(tssn.T, err, "DBUtil should initialize properly.")
	err = tssn.dbu.Clear()
	assert.Nil(tssn.T, err)
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
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_MOL)
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
