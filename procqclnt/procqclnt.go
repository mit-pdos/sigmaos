// Package proqqclnt implements the client-side of the procq scheduler
package procqclnt

import (
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

func (pqc *ProcQClnt) chooseProcQ(pid sp.Tpid) (string, error) {
	s := time.Now()
	pqId, err := pqc.urpcc.Random()
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt get ProcQ[%v] latency: %v", pid, pqId, time.Since(s))
	return pqId, err
}

// Enqueue a proc on the procq. Returns the ID of the kernel that is running
// the proc.
func (pqc *ProcQClnt) Enqueue(p *proc.Proc) (string, error) {
	pqID, err := pqc.chooseProcQ(p.GetPid())
	if err != nil {
		return NOT_ENQ, err
	}
	s := time.Now()
	rpcc, err := pqc.urpcc.GetClnt(pqID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get procq clnt: %v", err)
		return NOT_ENQ, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt make clnt %v %v", p.GetPid(), pqID, time.Since(s))
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
// receives a response, or once there is an error.
func (pqc *ProcQClnt) GetProc(callerKernelID string, freeMem proc.Tmem, bias bool) (proc.Tmem, uint32, bool, error) {
	// Retry until successful.
	for {
		var pqID string
		// Optionally bias the choice of procq to the caller's kernel
		if bias {
			pqID = callerKernelID
		} else {
			var err error
			pqID, err = pqc.urpcc.Random()
			if err != nil {
				db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get random: %v", err)
				return 0, 0, false, err
			}
		}
		rpcc, err := pqc.urpcc.GetClnt(pqID)
		if err != nil {
			db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get procq clnt: %v", err)
			return 0, 0, false, err
		}
		req := &proto.GetProcRequest{
			KernelID: callerKernelID,
			Mem:      uint32(freeMem),
		}
		res := &proto.GetProcResponse{}
		if err := rpcc.RPC("ProcQ.GetProc", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "ProcQ.GetProc %v err %v", callerKernelID, err)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
				pqc.urpcc.UnregisterSrv(pqID)
				continue
			}
			return 0, 0, false, err
		}
		db.DPrintf(db.PROCQCLNT, "GetProc success? %v", res.OK)
		return proc.Tmem(res.Mem), res.QLen, res.OK, nil
	}
}

func (pqc *ProcQClnt) GetQueueStats(nsample int) (map[sp.Trealm]int, error) {
	sampled := make(map[string]bool)
	qstats := make(map[sp.Trealm]int)
	for i := 0; i < nsample; i++ {
		pqID, err := pqc.urpcc.Random()
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv: %v", err)
			return nil, err
		}
		// Don't double-sample
		if sampled[pqID] {
			continue
		}
		sampled[pqID] = true
		rpcc, err := pqc.urpcc.GetClnt(pqID)
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv clnt: %v", err)
			return nil, err
		}
		req := &proto.GetStatsRequest{}
		res := &proto.GetStatsResponse{}
		if err := rpcc.RPC("ProcQ.GetStats", req, res); err != nil {
			db.DPrintf(db.ERROR, "Can't get stats: %v", err)
			return nil, err
		}
		for rstr, l := range res.Nqueued {
			r := sp.Trealm(rstr)
			if _, ok := qstats[r]; !ok {
				qstats[r] = 0
			}
			qstats[r] += int(l)
		}
	}
	return qstats, nil
}

func (pqc *ProcQClnt) StopWatching() {
	pqc.urpcc.StopWatching()
}
