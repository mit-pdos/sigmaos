package clnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/sched/msched/proc/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ProcClnt struct {
	mu    sync.Mutex
	cond  *sync.Cond
	done  bool
	pid   sp.Tpid
	kclnt *kernelclnt.KernelClnt
	*rpcclnt.RPCClnt
	realm    sp.Trealm
	ptype    proc.Ttype
	cpuShare Tshare
}

func NewProcClnt(pid sp.Tpid, rpcc *rpcclnt.RPCClnt, kclnt *kernelclnt.KernelClnt) *ProcClnt {
	pc := &ProcClnt{
		pid:      pid,
		kclnt:    kclnt,
		RPCClnt:  rpcc,
		realm:    sp.NOT_SET,
		ptype:    proc.T_LC,
		cpuShare: 0,
	}
	pc.cond = sync.NewCond(&pc.mu)
	go pc.monitorCPUShares()
	return pc
}

func (clnt *ProcClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.cpuShare)
}

func (clnt *ProcClnt) GetPid() sp.Tpid {
	return clnt.pid
}

func (clnt *ProcClnt) RunProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	req := &proto.RunReq{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunRep{}
	if err := clnt.RPC("ProcRPCSrv.Run", req, res); serr.IsErrorSession(err) {
		return err, nil
	} else {
		db.DPrintf(db.PROCDMGR_ERR, "Err child %v", err)
		return nil, err
	}
}

func (clnt *ProcClnt) WarmProcd(pid sp.Tpid, realm sp.Trealm, prog string, s3secret *sp.SecretProto, namedEP *sp.Tendpoint, path []string) (uprocErr error, childErr error) {
	req := &proto.WarmBinReq{
		RealmStr:           realm.String(),
		Program:            prog,
		SigmaPath:          path,
		PidStr:             pid.String(),
		S3Secret:           s3secret,
		NamedEndpointProto: namedEP.GetProto(),
	}
	res := &proto.RunRep{}
	if err := clnt.RPC("ProcRPCSrv.WarmProcd", req, res); serr.IsErrorSession(err) {
		return err, nil
	} else {
		return nil, err
	}
}

func (clnt *ProcClnt) monitorCPUShares() {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()

	prevCPUShareSetting := Tshare(0)
	for !clnt.done {
		if clnt.cpuShare != prevCPUShareSetting {
			// TODO: set shares
			prevCPUShareSetting = clnt.cpuShare
			if err := clnt.kclnt.SetCPUShares(clnt.pid, int64(clnt.cpuShare)); err != nil {
				db.DPrintf(db.PROCDMGR, "Error SetCPUShares[%v] %v", clnt.pid, err)
			}
		}
		clnt.cond.Wait()
	}
}

func (clnt *ProcClnt) Stop() {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()

	clnt.done = true
	clnt.cond.Signal()
}

func (clnt *ProcClnt) GetCPUShare() Tshare {
	return clnt.cpuShare
}

func (clnt *ProcClnt) SetCPUShare(share Tshare) {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()

	clnt.cpuShare = share
	clnt.cond.Signal()
}
