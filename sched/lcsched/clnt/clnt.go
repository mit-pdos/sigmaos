package lcschedclnt

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

type LCSchedClnt struct {
	*fslib.FsLib
	rpcdc *shardedsvcrpcclnt.ShardedSvcRPCClnt
}

func NewLCSchedClnt(fsl *fslib.FsLib) *LCSchedClnt {
	return &LCSchedClnt{
		FsLib: fsl,
		rpcdc: shardedsvcrpcclnt.NewShardedSvcRPCClnt(fsl, sp.LCSCHED, db.LCSCHEDCLNT, db.LCSCHEDCLNT_ERR),
	}
}

// Enqueue a proc on the lcsched. Returns the ID of the kernel that is running
// the proc.
func (lcs *LCSchedClnt) Enqueue(p *proc.Proc) (string, error) {
	start := time.Now()
	pqID, err := lcs.rpcdc.WaitTimedRoundRobin()
	if err != nil {
		return NOT_ENQ, err
	}
	perf.LogSpawnLatency("LCSchedClnt.Enqueue.WaitTimed", p.GetPid(), p.GetSpawnTime(), start)
	start = time.Now()
	rpcc, err := lcs.rpcdc.GetClnt(pqID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get lcsched clnt: %v", err)
		return NOT_ENQ, err
	}
	perf.LogSpawnLatency("LCSchedClnt.Enqueue.GetClnt", p.GetPid(), p.GetSpawnTime(), start)
	req := &proto.EnqueueReq{
		ProcProto: p.GetProto(),
	}
	res := &proto.EnqueueRep{}
	start = time.Now()
	if err := rpcc.RPC("LCSched.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "LCSched.Enqueue err %v", err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.ALWAYS, "Invalidate entry %v", pqID)
			lcs.rpcdc.InvalidateEntry(pqID)
		}
		return NOT_ENQ, err
	}
	perf.LogSpawnLatency("LCSchedClnt.Enqueue.RPC", p.GetPid(), p.GetSpawnTime(), start)
	db.DPrintf(db.LCSCHEDCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.MSchedID, nil
}

func (lcs *LCSchedClnt) StopWatching() {
	lcs.rpcdc.StopWatching()
}
