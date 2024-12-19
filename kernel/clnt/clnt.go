package clnt

import (
	"sigmaos/kernel/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type KernelClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewKernelClnt(fsl *fslib.FsLib, pn string) (*KernelClnt, error) {
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, pn)
	if err != nil {
		return nil, err
	}
	return &KernelClnt{fsl, rpcc}, nil
}

func (kc *KernelClnt) Boot(s string, args, env []string) (sp.Tpid, error) {
	return kc.BootInRealm(sp.ROOTREALM, s, args, env)
}

func (kc *KernelClnt) BootInRealm(realm sp.Trealm, s string, args, env []string) (sp.Tpid, error) {
	return bootInRealm(kc.rpcc, realm, s, args, env)
}

func (kc *KernelClnt) SetCPUShares(pid sp.Tpid, shares int64) error {
	var res proto.SetCPUSharesRep
	req := &proto.SetCPUSharesReq{PidStr: pid.String(), Shares: shares}
	return kc.rpcc.RPC("KernelSrv.SetCPUShares", req, &res)
}

func (kc *KernelClnt) GetCPUUtil(pid sp.Tpid) (float64, error) {
	var res proto.GetKernelSrvCPUUtilRep
	req := &proto.GetKernelSrvCPUUtilReq{PidStr: pid.String()}
	err := kc.rpcc.RPC("KernelSrv.GetCPUUtil", req, &res)
	if err != nil {
		return 0.0, err
	}
	return res.Util, nil
}

func (kc *KernelClnt) Kill(s string) error {
	var res proto.KillRep
	req := &proto.KillReq{Name: s}
	return kc.rpcc.RPC("KernelSrv.Kill", req, &res)
}

func (kc *KernelClnt) Shutdown() error {
	var res proto.ShutdownRep
	req := &proto.ShutdownReq{}
	return kc.rpcc.RPC("KernelSrv.Shutdown", req, &res)
}

func evictKernelProc(rpcc *rpcclnt.RPCClnt, pid sp.Tpid) error {
	var res proto.EvictKernelProcRep
	req := &proto.EvictKernelProcReq{PidStr: pid.String()}
	err := rpcc.RPC("KernelSrv.EvictKernelProc", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func bootInRealm(rpcc *rpcclnt.RPCClnt, realm sp.Trealm, s string, args, env []string) (sp.Tpid, error) {
	var res proto.BootRep
	req := &proto.BootReq{
		Name:     s,
		RealmStr: realm.String(),
		Args:     args,
		Env:      env,
	}
	err := rpcc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return sp.NO_PID, err
	}
	return sp.Tpid(res.PidStr), nil
}
