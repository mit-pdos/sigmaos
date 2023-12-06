package uprocclnt

import (
	"fmt"

	"sigmaos/proc"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

const (
	NOT_SET = "NOT_SET"
)

type UprocdClnt struct {
	pid sp.Tpid
	*rpcclnt.RPCClnt
	realm sp.Trealm
	ptype proc.Ttype
	share Tshare
}

func NewUprocdClnt(pid sp.Tpid, rpcc *rpcclnt.RPCClnt) *UprocdClnt {
	return &UprocdClnt{
		pid:     pid,
		RPCClnt: rpcc,
		realm:   NOT_SET,
		ptype:   proc.T_LC,
		share:   0,
	}
}

func (clnt *UprocdClnt) AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error {
	clnt.ptype = ptype
	req := &proto.AssignRequest{
		RealmStr: realm.String(),
	}
	res := &proto.AssignResult{}
	return clnt.RPC("UprocSrv.Assign", req, res)
}

func (clnt *UprocdClnt) RunProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	req := &proto.RunRequest{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunResult{}
	if err := clnt.RPC("UprocSrv.Run", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		return nil, err
	}
}

func (clnt *UprocdClnt) CheckpointProc(uproc *proc.Proc) (chkptLoc string, osPid int, childErr error) {
	// run and exit do resource accounting and share rebalancing for the
	// uprocds.
	req := &proto.CheckpointPidRequest{
		PidStr: uproc.ProcEnvProto.PidStr,
	}
	res := &proto.CheckpointPidResult{}
	if err := clnt.RPC("UprocSrv.Checkpoint", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return "", -1, err
	} else {
		return res.CheckpointLocation, int(res.OsPid), nil
	}
}

func (clnt *UprocdClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.share)
}
