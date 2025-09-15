package srv

import (
	"fmt"

	"sigmaos/apps/epcache"
	epclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	SRV_MCPU proc.Tmcpu = 1000
)

type EPCacheJob struct {
	sc      *sigmaclnt.SigmaClnt
	srvProc *proc.Proc
	Clnt    *epclnt.EndpointCacheClnt
}

func NewEPCacheJob(sc *sigmaclnt.SigmaClnt) (*EPCacheJob, error) {
	j := &EPCacheJob{
		sc:      sc,
		srvProc: proc.NewProc("epcached", []string{}),
	}
	j.srvProc.SetMcpu(SRV_MCPU)
	if err := sc.Spawn(j.srvProc); err != nil {
		db.DPrintf(db.TEST, "Err Spawn EPCache: %v", err)
		return nil, err
	}
	if err := j.sc.WaitStart(j.srvProc.GetPid()); err != nil {
		db.DPrintf(db.TEST, "Err WaitStart EPCache: %v", err)
		return nil, err
	}
	clnt, err := epclnt.NewEndpointCacheClnt(j.sc.FsLib)
	if err != nil {
		db.DPrintf(db.TEST, "Err NewClnt: %v", err)
		return nil, err
	}
	j.Clnt = clnt
	return j, nil
}

func (j *EPCacheJob) GetSrvEP() (*sp.Tendpoint, error) {
	// Read the endpoint of the endpoint cache server
	epcsrvEPB, err := j.sc.GetFile(epcache.EPCACHE)
	if err != nil {
		return nil, err
	}
	epcsrvEP, err := sp.NewEndpointFromBytes(epcsrvEPB)
	if err != nil {
		return nil, err
	}
	return epcsrvEP, nil
}

func (j *EPCacheJob) Stop() error {
	if err := j.sc.Evict(j.srvProc.GetPid()); err != nil {
		return err
	}
	status, err := j.sc.WaitExit(j.srvProc.GetPid())
	if err != nil {
		db.DPrintf(db.TEST, "err WaitExit: %v", err)
		return err
	}
	if err == nil && !status.IsStatusEvicted() {
		db.DPrintf(db.TEST, "Err wrong status: %v", status)
		return fmt.Errorf("Wrong status: %v", status)
	}
	db.DPrintf(db.TEST, "Stopped srv")
	return nil
}
