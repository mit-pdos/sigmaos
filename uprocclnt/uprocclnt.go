package uprocclnt

import (
	"fmt"

	// db "sigmaos/debug"
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

type UprocSrv interface {
	Lookup(pid int, prog string) (*sp.Tstat, error)
	Fetch(pid, cid int, prog string, sz sp.Tsize) (sp.Tsize, error)
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

func (clnt *UprocdClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.share)
}

func (clnt *UprocdClnt) GetPid() sp.Tpid {
	return clnt.pid
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

func (clnt *UprocdClnt) WarmProc(pid sp.Tpid, realm sp.Trealm, prog string, s3secret *sp.SecretProto, namedEP *sp.Tendpoint, path []string) (uprocErr error, childErr error) {
	req := &proto.WarmBinRequest{
		RealmStr:           realm.String(),
		Program:            prog,
		SigmaPath:          path,
		PidStr:             pid.String(),
		S3Secret:           s3secret,
		NamedEndpointProto: namedEP.GetProto(),
	}
	res := &proto.RunResult{}
	if err := clnt.RPC("UprocSrv.WarmProc", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		return nil, err
	}
}
