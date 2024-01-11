package lcschedclnt

import (
	"errors"
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	pqproto "sigmaos/procqsrv/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/unionrpcclnt"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type LCSchedClnt struct {
	*fslib.FsLib
	urpcc *unionrpcclnt.UnionRPCClnt
}

func NewLCSchedClnt(fsl *fslib.FsLib, provider sp.Tprovider) *LCSchedClnt {
	pn := path.Join(sp.LCSCHED, provider.TproviderToDir())
	return &LCSchedClnt{
		FsLib: fsl,
		urpcc: unionrpcclnt.NewUnionRPCClnt(fsl, pn, db.LCSCHEDCLNT, db.LCSCHEDCLNT_ERR),
	}
}

// Enqueue a proc on the lcsched. Returns the ID of the kernel that is running
// the proc.
func (lcs *LCSchedClnt) Enqueue(p *proc.Proc) (string, error) {
	lcs.urpcc.UpdateSrvs(false)
	pqID, err := lcs.urpcc.NextSrv()
	if err != nil {
		return NOT_ENQ, errors.New("No lcscheds available")
	}
	rpcc, err := lcs.urpcc.GetClnt(pqID)
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
			lcs.urpcc.UnregisterSrv(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.LCSCHEDCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.KernelID, nil
}
