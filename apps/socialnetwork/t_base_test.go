package socialnetwork_test

import (
	"github.com/stretchr/testify/assert"
	sn "sigmaos/apps/socialnetwork"
	dbg "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/util/rand"
	"sigmaos/test"
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

func newTstateSN(t *test.Tstate, srvs []sn.Srv, nsrv int) (*TstateSN, error) {
	var err error
	tssn := &TstateSN{}
	tssn.jobname = rand.String(8)
	tssn.Tstate = t
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
	if !assert.Nil(tssn.T, err, "Err create mongoc: %v", err) {
		return tssn, err
	}
	err = tssn.dbu.Clear()
	if !assert.Nil(tssn.T, err, "Err clear mongoc: %v", err) {
		return tssn, err
	}
	return tssn, nil
}

func (tssn *TstateSN) Shutdown() error {
	if stopErr := tssn.snCfg.Stop(); stopErr != nil {
		return stopErr
	}
	return tssn.Tstate.Shutdown()
}
