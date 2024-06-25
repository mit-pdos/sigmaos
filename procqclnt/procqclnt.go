// Package proqqclnt implements the client-side of the procq scheduler
package procqclnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procqsrv/proto"
	"sigmaos/rpcdirclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type ProcQClnt struct {
	*fslib.FsLib
	rpcdc  *rpcdirclnt.RPCDirClnt
	pseqno *syncmap.SyncMap[string, *ProcSeqno]
}

func NewProcQClnt(fsl *fslib.FsLib) *ProcQClnt {
	return &ProcQClnt{
		FsLib:  fsl,
		rpcdc:  rpcdirclnt.NewRPCDirClnt(fsl, sp.PROCQ, db.PROCQCLNT, db.PROCQCLNT_ERR),
		pseqno: syncmap.NewSyncMap[string, *ProcSeqno](),
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
func (pqc *ProcQClnt) Enqueue(p *proc.Proc) (string, string, uint64, error) {
	pqID, err := pqc.chooseProcQ(p.GetPid())
	if err != nil {
		return NOT_ENQ, NOT_ENQ, 0, err
	}
	s := time.Now()
	rpcc, err := pqc.rpcdc.GetClnt(pqID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get procq clnt: %v", err)
		return NOT_ENQ, NOT_ENQ, 0, err
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
		return NOT_ENQ, NOT_ENQ, 0, err
	}
	db.DPrintf(db.PROCQCLNT, "[%v] Enqueued Proc %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcQClnt client-side RPC latency %v", p.GetPid(), time.Since(s))
	return pqID, res.ScheddID, res.ProcSeqno, nil
}

// Get a proc (passing in the kernelID of the caller). Will only return once
// receives a response, or once there is an error.
func (pqc *ProcQClnt) GetProc(callerKernelID string, freeMem proc.Tmem, bias bool) (*proc.Proc, uint32, bool, error) {
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
				return nil, 0, false, err
			}
		}
		pseqno, _ := pqc.pseqno.AllocNew(pqID, func(string) *ProcSeqno {
			return NewProcSeqno()
		})
		rpcc, err := pqc.rpcdc.GetClnt(pqID)
		if err != nil {
			db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get procq clnt: %v", err)
			return nil, 0, false, err
		}
		procSeqno := pseqno.GetNext()
		req := &proto.GetProcRequest{
			KernelID:      callerKernelID,
			Mem:           uint32(freeMem),
			NextProcSeqno: procSeqno,
		}
		res := &proto.GetProcResponse{}
		if err := rpcc.RPC("ProcQ.GetProc", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "ProcQ.GetProc %v err %v", callerKernelID, err)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Invalidate entry %v", pqID)
				pqc.rpcdc.InvalidateEntry(pqID)
				continue
			}
			return nil, 0, false, err
		}
		db.DPrintf(db.PROCQCLNT, "GetProc success? %v", res.OK)
		var p *proc.Proc
		if res.OK {
			p = proc.NewProcFromProto(res.GetProcProto())
			pqc.GotProc(pqID, procSeqno)
		}
		return p, res.QLen, res.OK, nil
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

// Note that a proc has been received, so the sequence number can be
// incremented
func (pqc *ProcQClnt) GotProc(pqID string, seqno uint64) {
	// schedd has successfully received a proc from procq pqID. Any clients which
	// want to wait on that proc can now expect the state for that proc to exist
	// at schedd. Set the seqno (which should be monotonically increasing) to
	// release the clients, and allow schedd to handle the wait.
	pseqno, _ := pqc.pseqno.AllocNew(pqID, func(string) *ProcSeqno {
		return NewProcSeqno()
	})
	pseqno.Got(seqno)
}

// Wait to hear about a proc from procq pqID.
func (pqc *ProcQClnt) WaitUntilGotProc(pqID string, seqno uint64) {
	pseqno, _ := pqc.pseqno.AllocNew(pqID, func(string) *ProcSeqno {
		return NewProcSeqno()
	})
	pseqno.WaitUntilGot(seqno)
}

func (pqc *ProcQClnt) StopWatching() {
	pqc.rpcdc.StopWatching()
}
