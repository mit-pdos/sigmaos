package lcschedclnt

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	shardedsvcrpcclnt "sigmaos/rpc/shardedsvc/clnt"
	"sigmaos/sched/besched/proto"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
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
	pqID, err := lcs.rpcdc.WaitTimedRoundRobin()
	if err != nil {
		return NOT_ENQ, err
	}
	rpcc, err := lcs.rpcdc.GetClnt(pqID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get lcsched clnt: %v", err)
		return NOT_ENQ, err
	}
	req := &proto.EnqueueReq{
		ProcProto: p.GetProto(),
	}
	res := &proto.EnqueueRep{}
	if err := rpcc.RPC("LCSched.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "LCSched.Enqueue err %v", err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.ALWAYS, "Invalidate entry %v", pqID)
			lcs.rpcdc.InvalidateEntry(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.LCSCHEDCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.MSchedID, nil
}

func (lcs *LCSchedClnt) StopWatching() {
	lcs.rpcdc.StopWatching()
}
