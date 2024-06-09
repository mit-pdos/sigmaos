package lcschedclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	pqproto "sigmaos/procqsrv/proto"
	"sigmaos/rpcdirclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type LCSchedClnt struct {
	*fslib.FsLib
	rpcdc *rpcdirclnt.RPCDirClnt
}

func NewLCSchedClnt(fsl *fslib.FsLib) *LCSchedClnt {
	return &LCSchedClnt{
		FsLib: fsl,
		rpcdc: rpcdirclnt.NewRPCDirClnt(fsl, sp.LCSCHED, db.LCSCHEDCLNT, db.LCSCHEDCLNT_ERR),
	}
}

// Enqueue a proc on the lcsched. Returns the ID of the kernel that is running
// the proc.
func (lcs *LCSchedClnt) Enqueue(p *proc.Proc) (string, error) {
	pqID, err := lcs.rpcdc.WaitRoundRobin()
	if err != nil {
		return NOT_ENQ, err
	}
	rpcc, err := lcs.rpcdc.GetClnt(pqID)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error: Can't get lcsched clnt: %v", err)
		return NOT_ENQ, err
	}
	req := &pqproto.EnqueueRequest{
		ProcProto: p.GetProto(),
	}
	res := &pqproto.EnqueueResponse{}
	if err := rpcc.RPC("LCSched.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "LCSched.Enqueue err %v", err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
			lcs.rpcdc.InvalidateEntry(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.LCSCHEDCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.KernelID, nil
}

func (lcs *LCSchedClnt) StopWatching() {
	lcs.rpcdc.StopWatching()
}
