package uprocclnt

import (
	"fmt"
	"time"

	db "sigmaos/debug"
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

func (clnt *UprocdClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.share)
}

func (clnt *UprocdClnt) GetPid() sp.Tpid {
	return clnt.pid
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

func (clnt *UprocdClnt) WarmProc(pid sp.Tpid, realm sp.Trealm, prog string, s3secret *sp.SecretProto, path []string) (uprocErr error, childErr error) {
	req := &proto.WarmBinRequest{
		RealmStr:  realm.String(),
		Program:   prog,
		SigmaPath: path,
		PidStr:    pid.String(),
		S3Secret:  s3secret,
	}
	res := &proto.RunResult{}
	if err := clnt.RPC("UprocSrv.WarmProc", req, res); serr.IsErrCode(err, serr.TErrUnreachable) {
		return err, nil
	} else {
		return nil, err
	}
}

func (clnt *UprocdClnt) Fetch(pn string, ck int, sz sp.Tsize, pid uint32) (sp.Tsize, error) {
	s := time.Now()
	req := &proto.FetchRequest{
		Prog:    pn,
		ChunkId: int32(ck),
		Size:    uint64(sz),
		Pid:     pid,
	}
	res := &proto.FetchResponse{}
	if err := clnt.RPC("UprocSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.ERROR, "UprocSrv.Fetch %v err %v", req, err)
		return 0, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocdclnt.Fetch ck %d %v", pn, ck, time.Since(s))
	return sp.Tsize(res.Size), nil
}

func (clnt *UprocdClnt) Lookup(pn string, pid uint32) (*sp.Stat, error) {
	s := time.Now()
	req := &proto.LookupRequest{
		Prog: pn,
		Pid:  pid,
	}
	res := &proto.LookupResponse{}
	if err := clnt.RPC("UprocSrv.Lookup", req, res); err != nil {
		db.DPrintf(db.ERROR, "UprocSrv.Lookup %v err %v", req, err)
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocdclnt.Lookup pid %d %v", pn, pid, time.Since(s))
	return sp.NewStatProto(res.Stat), nil
}

func (clnt *UprocdClnt) Assign() error {
	req := &proto.AssignRequest{}
	res := &proto.FetchResponse{}
	if err := clnt.RPC("UprocSrv.Assign", req, res); err != nil {
		return err
	}
	return nil
}
