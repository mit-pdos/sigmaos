package clnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/sched/msched/proc/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ProcClnt struct {
	pid sp.Tpid
	*rpcclnt.RPCClnt
	realm sp.Trealm
	ptype proc.Ttype
	share Tshare
}

func NewProcClnt(pid sp.Tpid, rpcc *rpcclnt.RPCClnt) *ProcClnt {
	return &ProcClnt{
		pid:     pid,
		RPCClnt: rpcc,
		realm:   sp.NOT_SET,
		ptype:   proc.T_LC,
		share:   0,
	}
}

func (clnt *ProcClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.share)
}

func (clnt *ProcClnt) GetPid() sp.Tpid {
	return clnt.pid
}

func (clnt *ProcClnt) RunProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	req := &proto.RunRequest{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunResult{}
	if err := clnt.RPC("ProcRPCSrv.Run", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		db.DPrintf(db.PROCDMGR_ERR, "Err child %v", err)
		return nil, err
	}
}

func (clnt *ProcClnt) CheckpointProc(pid sp.Tpid, pn string) (uprocErr error, childErr error) {
	req := &proto.CheckpointProcRequest{
		PidStr:   pid.String(),
		PathName: pn,
	}
	res := &proto.CheckpointProcResponse{}
	if err := clnt.RPC("ProcRPCSrv.Checkpoint", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		return nil, err
	}
}

func (clnt *ProcClnt) WarmProcd(pid sp.Tpid, realm sp.Trealm, prog string, s3secret *sp.SecretProto, namedEP *sp.Tendpoint, path []string) (uprocErr error, childErr error) {
	req := &proto.WarmBinRequest{
		RealmStr:           realm.String(),
		Program:            prog,
		SigmaPath:          path,
		PidStr:             pid.String(),
		S3Secret:           s3secret,
		NamedEndpointProto: namedEP.GetProto(),
	}
	res := &proto.RunResult{}
	if err := clnt.RPC("ProcRPCSrv.WarmProcd", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		return nil, err
	}
}
