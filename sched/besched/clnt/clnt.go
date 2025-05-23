// Package [beschedclnt] implements the client-side of the besched scheduler
package clnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	shardedsvcrpcclnt "sigmaos/rpc/shardedsvc/clnt"
	"sigmaos/sched/besched/proto"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type nextSeqnoFn func(string) *proc.ProcSeqno

type BESchedClnt struct {
	*fslib.FsLib
	rpcdc     *shardedsvcrpcclnt.ShardedSvcRPCClnt
	nextSeqno nextSeqnoFn
}

func NewBESchedClnt(fsl *fslib.FsLib) *BESchedClnt {
	return NewBESchedClntMSched(fsl, nil, nil)
}

func NewBESchedClntMSched(fsl *fslib.FsLib, nextEpoch shardedsvcrpcclnt.AllocFn, nextSeqno nextSeqnoFn) *BESchedClnt {
	return &BESchedClnt{
		FsLib:     fsl,
		rpcdc:     shardedsvcrpcclnt.NewShardedSvcRPCClntAllocFn(fsl, sp.BESCHED, db.BESCHEDCLNT, db.BESCHEDCLNT_ERR, nextEpoch),
		nextSeqno: nextSeqno,
	}
}

func (besc *BESchedClnt) chooseBESched(p *proc.Proc) (string, error) {
	s := time.Now()
	besId, err := besc.rpcdc.WaitTimedRandomEntry()
	perf.LogSpawnLatency("BESchedClnt.GetProc.chooseBESched", p.GetPid(), p.GetSpawnTime(), s)
	return besId, err
}

// Enqueue a proc on the besched. Returns the ID of the kernel that is running
// the proc.
func (besc *BESchedClnt) Enqueue(p *proc.Proc) (string, *proc.ProcSeqno, error) {
	start := time.Now()
	defer func(start time.Time) {
		perf.LogSpawnLatency("BESchedClnt.Enqueue", p.GetPid(), p.GetSpawnTime(), start)
	}(start)
	besID, err := besc.chooseBESched(p)
	if err != nil {
		return NOT_ENQ, nil, err
	}
	s := time.Now()
	rpcc, err := besc.rpcdc.GetClnt(besID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get besched clnt: %v", err)
		return NOT_ENQ, nil, err
	}
	perf.LogSpawnLatency("BESchedClnt.Enqueue.GetClnt", p.GetPid(), p.GetSpawnTime(), s)
	req := &proto.EnqueueReq{
		ProcProto: p.GetProto(),
	}
	res := &proto.EnqueueRep{}
	s = time.Now()
	if err := rpcc.RPC("BESched.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "BESched.Enqueue err %v", err)
		if serr.IsErrorUnreachable(err) {
			db.DPrintf(db.ALWAYS, "Invalidate entry %v", besID)
			besc.rpcdc.InvalidateEntry(besID)
		}
		return NOT_ENQ, nil, err
	}
	db.DPrintf(db.BESCHEDCLNT, "[%v] Enqueued Proc %v", p.GetRealm(), p)
	perf.LogSpawnLatency("BESchedClnt.Enqueue RPC", p.GetPid(), p.GetSpawnTime(), s)
	return res.MSchedID, res.ProcSeqno, nil
}

// Get a proc (passing in the kernelID of the caller). Will only return once
// receives a response, or once there is an error.
func (besc *BESchedClnt) GetProc(callerKernelID string, freeMem proc.Tmem, bias bool) (*proc.Proc, *proc.ProcSeqno, uint32, bool, error) {
	// Retry until successful.
	for {
		var besID string
		// Optionally bias the choice of besched to the caller's kernel
		if bias {
			besID = callerKernelID
		} else {
			var err error
			besID, err = besc.rpcdc.WaitTimedRandomEntry()
			if err != nil {
				db.DPrintf(db.BESCHEDCLNT_ERR, "Error: Can't get random: %v", err)
				return nil, nil, 0, false, err
			}
		}
		rpcc, err := besc.rpcdc.GetClnt(besID)
		if err != nil {
			db.DPrintf(db.BESCHEDCLNT_ERR, "Error: Can't get besched clnt: %v", err)
			return nil, nil, 0, false, err
		}
		procSeqno := besc.nextSeqno(besID)
		req := &proto.GetProcReq{
			KernelID:  callerKernelID,
			Mem:       uint32(freeMem),
			ProcSeqno: procSeqno,
		}
		res := &proto.GetProcRep{}
		if err := rpcc.RPC("BESched.GetProc", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "BESched.GetProc %v err %v", callerKernelID, err)
			if serr.IsErrorUnreachable(err) {
				db.DPrintf(db.ALWAYS, "Invalidate entry %v", besID)
				besc.rpcdc.InvalidateEntry(besID)
				continue
			}
			return nil, nil, 0, false, err
		}
		db.DPrintf(db.BESCHEDCLNT, "GetProc success? %v", res.OK)
		var p *proc.Proc
		if res.OK {
			p = proc.NewProcFromProto(res.GetProcProto())
		}
		return p, procSeqno, res.QLen, res.OK, nil
	}
}

func (besc *BESchedClnt) GetQueueStats(nsample int) (map[sp.Trealm]int, error) {
	sampled := make(map[string]bool)
	qstats := make(map[sp.Trealm]int)
	for i := 0; i < nsample; i++ {
		besID, err := besc.rpcdc.WaitTimedRandomEntry()
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv: %v", err)
			return nil, err
		}
		// Don't double-sample
		if sampled[besID] {
			continue
		}
		sampled[besID] = true
		rpcc, err := besc.rpcdc.GetClnt(besID)
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv clnt: %v", err)
			return nil, err
		}
		req := &proto.GetStatsReq{}
		res := &proto.GetStatsRep{}
		if err := rpcc.RPC("BESched.GetStats", req, res); err != nil {
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

func (besc *BESchedClnt) StopWatching() {
	besc.rpcdc.StopWatching()
}
