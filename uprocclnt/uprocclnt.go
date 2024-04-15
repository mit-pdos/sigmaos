package uprocclnt

import (
	"fmt"

	"sigmaos/proc"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
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
		realm:   sp.NOT_SET,
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

func (clnt *UprocdClnt) WarmProc(realm sp.Trealm, prog, buildTag string) (uprocErr error, childErr error) {
	req := &proto.WarmBinRequest{
		RealmStr: realm.String(),
		Program:  prog,
		BuildTag: buildTag,
	}
	res := &proto.RunResult{}
	if err := clnt.RPC("UprocSrv.WarmProc", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		return nil, err
	}
}

func (clnt *UprocdClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.share)
}
