package kernelclnt

import (
	"sigmaos/fslib"
	"sigmaos/kernelsrv/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
)

type KernelClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewKernelClnt(fsl *fslib.FsLib, pn string) (*KernelClnt, error) {
	rpcc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{fsl}, pn)
	if err != nil {
		return nil, err
	}
	return &KernelClnt{fsl, rpcc}, nil
}

func (kc *KernelClnt) Boot(s string, args []string) (sp.Tpid, error) {
	return kc.BootInRealm(sp.ROOTREALM, s, args)
}

func (kc *KernelClnt) BootInRealm(realm sp.Trealm, s string, args []string) (sp.Tpid, error) {
	return bootInRealm(kc.rpcc, realm, s, args)
}

func (kc *KernelClnt) SetCPUShares(pid sp.Tpid, shares int64) error {
	var res proto.SetCPUSharesResponse
	req := &proto.SetCPUSharesRequest{PidStr: pid.String(), Shares: shares}
	return kc.rpcc.RPC("KernelSrv.SetCPUShares", req, &res)
}

func (kc *KernelClnt) GetCPUUtil(pid sp.Tpid) (float64, error) {
	var res proto.GetKernelSrvCPUUtilResponse
	req := &proto.GetKernelSrvCPUUtilRequest{PidStr: pid.String()}
	err := kc.rpcc.RPC("KernelSrv.GetCPUUtil", req, &res)
	if err != nil {
		return 0.0, err
	}
	return res.Util, nil
}

func (kc *KernelClnt) Kill(s string) error {
	var res proto.KillResult
	req := &proto.KillRequest{Name: s}
	return kc.rpcc.RPC("KernelSrv.Kill", req, &res)
}

func (kc *KernelClnt) Shutdown() error {
	var res proto.ShutdownResult
	req := &proto.ShutdownRequest{}
	return kc.rpcc.RPC("KernelSrv.Shutdown", req, &res)
}

func evictKernelProc(rpcc *rpcclnt.RPCClnt, pid sp.Tpid) error {
	var res proto.EvictKernelProcResponse
	req := &proto.EvictKernelProcRequest{PidStr: pid.String()}
	err := rpcc.RPC("KernelSrv.EvictKernelProc", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func bootInRealm(rpcc *rpcclnt.RPCClnt, realm sp.Trealm, s string, args []string) (sp.Tpid, error) {
	var res proto.BootResult
	req := &proto.BootRequest{
		Name:     s,
		RealmStr: realm.String(),
		Args:     args,
	}
	err := rpcc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return sp.NO_PID, err
	}
	return sp.Tpid(res.PidStr), nil
}
