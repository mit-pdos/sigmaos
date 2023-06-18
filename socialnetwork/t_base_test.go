package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	"sigmaos/rand"
	sn "sigmaos/socialnetwork"
	dbg "sigmaos/debug"
	"sigmaos/linuxsched"
	"github.com/stretchr/testify/assert"
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
	if test.Start {
		nMoreKernel := ((len(srvs)*2 + NSHARD*2) - 1)  / int(linuxsched.NCores)
		if nMoreKernel > 0 {
			dbg.DPrintf(dbg.ALWAYS, "(%v - 1) / %v = %v more kernels are needed",
				len(srvs)*2 + NSHARD*2, linuxsched.NCores, nMoreKernel)
			err = tssn.BootNode(nMoreKernel)
			assert.Nil(tssn.T, err)
		}
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
