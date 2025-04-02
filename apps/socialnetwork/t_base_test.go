package socialnetwork_test

import (
	"github.com/stretchr/testify/assert"
	sn "sigmaos/apps/socialnetwork"
	dbg "sigmaos/debug"
	"sigmaos/test"
	linuxsched "sigmaos/util/linux/sched"
	"sigmaos/util/rand"
)

const (
	NCACHESRV = 3
)

type TstateSN struct {
	mrts    *test.MultiRealmTstate
	jobname string
	snCfg   *sn.SocialNetworkConfig
	dbu     *sn.DBUtil
}

func newTstateSN(mrts *test.MultiRealmTstate, srvs []sn.Srv, nsrv int) (*TstateSN, error) {
	var err error
	tssn := &TstateSN{}
	tssn.jobname = rand.Name()
	tssn.mrts = mrts
	if test.Start {
		nMoreKernel := ((len(srvs)*2 + NCACHESRV) - 1) / int(linuxsched.GetNCores())
		if nMoreKernel > 0 {
			dbg.DPrintf(dbg.ALWAYS, "(%v - 1) / %v = %v more kernels are needed",
				len(srvs)*2+NCACHESRV, linuxsched.GetNCores(), nMoreKernel)
			err = tssn.mrts.GetRealm(test.REALM1).BootNode(nMoreKernel)
			assert.Nil(tssn.mrts.T, err)
		}
	}
	tssn.snCfg, err = sn.NewConfig(tssn.mrts.GetRealm(test.REALM1).SigmaClnt, tssn.jobname, srvs, nsrv, false)
	assert.Nil(tssn.mrts.T, err, "config should initialize properly.")
	tssn.dbu, err = sn.NewDBUtil(tssn.mrts.GetRealm(test.REALM1).SigmaClnt)
	if !assert.Nil(tssn.mrts.T, err, "Err create mongoc: %v", err) {
		return tssn, err
	}
	err = tssn.dbu.Clear()
	if !assert.Nil(tssn.mrts.T, err, "Err clear mongoc: %v", err) {
		return tssn, err
	}
	return tssn, nil
}

func (tssn *TstateSN) Shutdown() error {
	return tssn.snCfg.Stop()
}
