// Package proqqclnt implements the client-side of the procq scheduler
package procqclnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procqsrv/proto"
	//	"sigmaos/rpc"
	"sigmaos/rpcdirclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type nextSeqnoFn func(string) *proc.ProcSeqno

type ProcQClnt struct {
	*fslib.FsLib
	rpcdc     *rpcdirclnt.RPCDirClnt
	nextSeqno nextSeqnoFn
}

func NewProcQClnt(fsl *fslib.FsLib) *ProcQClnt {
	return NewProcQClntSchedd(fsl, nil, nil)
}

func NewProcQClntSchedd(fsl *fslib.FsLib, nextEpoch rpcdirclnt.AllocFn, nextSeqno nextSeqnoFn) *ProcQClnt {
	return &ProcQClnt{
		FsLib:     fsl,
		rpcdc:     rpcdirclnt.NewRPCDirClntAllocFn(fsl, sp.PROCQ, db.PROCQCLNT, db.PROCQCLNT_ERR, nextEpoch),
		nextSeqno: nextSeqno,
	}
}

func (pqc *ProcQClnt) chooseProcQ(pid sp.Tpid) (string, error) {
	s := time.Now()
	pqId, err := pqc.rpcdc.WaitTimedRandomEntry()
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt get ProcQ[%v] latency: %v", pid, pqId, time.Since(s))
	return pqId, err
}

// Enqueue a proc on the procq. Returns the ID of the kernel that is running
// the proc.
func (pqc *ProcQClnt) Enqueue(p *proc.Proc) (string, *proc.ProcSeqno, error) {
	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.SPAWN_LAT, "[%v] procqclnt.Enqueue: %v", p.GetPid(), time.Since(start))
	}(start)
	pqID, err := pqc.chooseProcQ(p.GetPid())
	if err != nil {
		return NOT_ENQ, nil, err
	}
	s := time.Now()
	rpcc, err := pqc.rpcdc.GetClnt(pqID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get procq clnt: %v", err)
		return NOT_ENQ, nil, err
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
			db.DPrintf(db.ALWAYS, "Invalidate entry %v", pqID)
			pqc.rpcdc.InvalidateEntry(pqID)
		}
		return NOT_ENQ, nil, err
	}
	db.DPrintf(db.PROCQCLNT, "[%v] Enqueued Proc %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt client-side RPC latency %v", p.GetPid(), time.Since(s))
	return res.ScheddID, res.ProcSeqno, nil
}

// Get a proc (passing in the kernelID of the caller). Will only return once
// receives a response, or once there is an error.
func (pqc *ProcQClnt) GetProc(callerKernelID string, freeMem proc.Tmem, bias bool) (*proc.Proc, *proc.ProcSeqno, uint32, bool, error) {
	// Retry until successful.
	for {
		var pqID string
		// Optionally bias the choice of procq to the caller's kernel
		if bias {
			pqID = callerKernelID
		} else {
			var err error
			pqID, err = pqc.rpcdc.WaitTimedRandomEntry()
			if err != nil {
				db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get random: %v", err)
				return nil, nil, 0, false, err
			}
		}
		rpcc, err := pqc.rpcdc.GetClnt(pqID)
		if err != nil {
			db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get procq clnt: %v", err)
			return nil, nil, 0, false, err
		}
		procSeqno := pqc.nextSeqno(pqID)
		req := &proto.GetProcRequest{
			KernelID:  callerKernelID,
			Mem:       uint32(freeMem),
			ProcSeqno: procSeqno,
		}
		res := &proto.GetProcResponse{}
		if err := rpcc.RPC("ProcQ.GetProc", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "ProcQ.GetProc %v err %v", callerKernelID, err)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Invalidate entry %v", pqID)
				pqc.rpcdc.InvalidateEntry(pqID)
				continue
			}
			return nil, nil, 0, false, err
		}
		db.DPrintf(db.PROCQCLNT, "GetProc success? %v", res.OK)
		var p *proc.Proc
		if res.OK {
			p = proc.NewProcFromProto(res.GetProcProto())
		}
		return p, procSeqno, res.QLen, res.OK, nil
	}
}

func (pqc *ProcQClnt) GetQueueStats(nsample int) (map[sp.Trealm]int, error) {
	sampled := make(map[string]bool)
	qstats := make(map[sp.Trealm]int)
	for i := 0; i < nsample; i++ {
		pqID, err := pqc.rpcdc.WaitTimedRandomEntry()
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv: %v", err)
			return nil, err
		}
		// Don't double-sample
		if sampled[pqID] {
			continue
		}
		sampled[pqID] = true
		rpcc, err := pqc.rpcdc.GetClnt(pqID)
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
	pqc.rpcdc.StopWatching()
}

// XXX
//func (pqc *ProcQClnt) GetRPCStats() (map[string]*rpc.RPCStatsSnapshot, error) {
//	snaps := make(map[string]*rpc.RPCStatsSnapshot)
//	srvs, err := pqc.rpcdc.GetEntries()
//	if err != nil {
//		db.DPrintf(db.ERROR, "Err GetEntries: %v", err)
//		return nil, err
//	}
//	for _, srv := range srvs {
//		clnt, err := pqc.rpcdc.GetClnt(srvID)
//		if err != nil {
//			db.DPrintf(db.ERROR, "Err GetClnt[%v]: %v", srvID, err)
//			return nil, err
//		}
//	}
//}
