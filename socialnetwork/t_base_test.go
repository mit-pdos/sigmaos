package socialnetwork_test

import (
	"github.com/stretchr/testify/assert"
	dbg "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/rand"
	sn "sigmaos/socialnetwork"
	"sigmaos/test"
	"testing"
)

const (
	NCACHESRV = 3
)

type TstateSN struct {
	*test.Tstate
	jobname string
	snCfg   *sn.SocialNetworkConfig
	dbu     *sn.DBUtil
}

func newTstateSN(t *testing.T, srvs []sn.Srv, nsrv int) *TstateSN {
	var err error
	tssn := &TstateSN{}
	tssn.jobname = rand.String(8)
	tssn.Tstate = test.NewTstateAll(t)
	if test.Start {
		nMoreKernel := ((len(srvs)*2 + NCACHESRV) - 1) / int(linuxsched.GetNCores())
		if nMoreKernel > 0 {
			dbg.DPrintf(dbg.ALWAYS, "(%v - 1) / %v = %v more kernels are needed",
				len(srvs)*2+NCACHESRV, linuxsched.GetNCores(), nMoreKernel)
			err = tssn.BootNode(nMoreKernel)
			assert.Nil(tssn.T, err)
		}
	}
	tssn.snCfg, err = sn.NewConfig(tssn.SigmaClnt, tssn.jobname, srvs, nsrv, false, test.Overlays)
	assert.Nil(tssn.T, err, "config should initialize properly.")
	tssn.dbu, err = sn.NewDBUtil(tssn.SigmaClnt)
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
