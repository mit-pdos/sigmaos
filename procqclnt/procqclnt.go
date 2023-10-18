package procqclnt

import (
	"errors"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procqsrv/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/unionrpcclnt"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type ProcQClnt struct {
	*fslib.FsLib
	urpcc *unionrpcclnt.UnionRPCClnt
}

func NewProcQClnt(fsl *fslib.FsLib) *ProcQClnt {
	return &ProcQClnt{
		FsLib: fsl,
		urpcc: unionrpcclnt.NewUnionRPCClnt(fsl, sp.PROCQ, db.PROCQCLNT, db.PROCQCLNT_ERR),
	}
}

// Enqueue a proc on the procq. Returns the ID of the kernel that is running
// the proc.
func (pqc *ProcQClnt) Enqueue(p *proc.Proc) (string, error) {
	s := time.Now()
	pqc.urpcc.UpdateSrvs(false)
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt updateProcQs %v", p.GetPid(), time.Since(s))
	s = time.Now()
	pqID, err := pqc.urpcc.NextSrv()
	if err != nil {
		return NOT_ENQ, errors.New("No procqs available")
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt get ProcQ[%v] latency: %v", p.GetPid(), pqID, time.Since(s))
	s = time.Now()
	rpcc, err := pqc.urpcc.GetClnt(pqID)
	if err != nil {
		db.DFatalf("Error: Can't get procq clnt: %v", err)
		return NOT_ENQ, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt make clnt %v", p.GetPid(), time.Since(s))
	req := &proto.EnqueueRequest{
		ProcProto: p.GetProto(),
	}
	res := &proto.EnqueueResponse{}
	s = time.Now()
	if err := rpcc.RPC("ProcQ.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "ProcQ.Enqueue err %v", err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
			pqc.urpcc.UnregisterSrv(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.PROCQCLNT, "[%v] Enqueued Proc %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt client-side RPC latency %v", p.GetPid(), time.Since(s))
	return res.KernelID, nil
}

// Get a proc (passing in the kernelID of the caller). Will only return once
// successful, or once there is an error.
func (pqc *ProcQClnt) GetProc(callerKernelID string) (bool, error) {
	pqc.urpcc.UpdateSrvs(false)
	// Retry until successful.
	for {
		pqID, err := pqc.urpcc.NextSrv()
		if err != nil {
			pqc.urpcc.UpdateSrvs(true)
			db.DPrintf(db.PROCQCLNT_ERR, "No procQs available: %v", err)
			continue
		}
		rpcc, err := pqc.urpcc.GetClnt(pqID)
		if err != nil {
			db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get procq clnt: %v", err)
			return false, err
		}
		req := &proto.GetProcRequest{
			KernelID: callerKernelID,
		}
		res := &proto.GetProcResponse{}
		if err := rpcc.RPC("ProcQ.GetProc", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "ProcQ.GetProc %v err %v", callerKernelID, err)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
				pqc.urpcc.UnregisterSrv(pqID)
				continue
			}
			return false, err
		}
		db.DPrintf(db.PROCQCLNT, "GetProc success? %v", res.OK)
		return res.OK, nil
	}
}
