package kernelclnt

import (
	"sigmaos/fslib"
	"sigmaos/kernelsrv/proto"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type KernelClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewKernelClnt(fsl *fslib.FsLib, pn string) (*KernelClnt, error) {
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{fsl}, pn)
	if err != nil {
		return nil, err
	}
	return &KernelClnt{fsl, rpcc}, nil
}

func (kc *KernelClnt) Boot(s string, args []string) (sp.Tpid, error) {
	var res proto.BootResult
	req := &proto.BootRequest{Name: s, Args: args}
	err := kc.rpcc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return sp.Tpid(""), err
	}
	return sp.Tpid(res.PidStr), nil
}

func (kc *KernelClnt) SetCPUShares(pid sp.Tpid, shares int64) error {
	var res proto.SetCPUSharesResponse
	req := &proto.SetCPUSharesRequest{PidStr: pid.String(), Shares: shares}
	return kc.rpcc.RPC("KernelSrv.SetCPUShares", req, &res)
}

func (kc *KernelClnt) AssignToRealm(pid sp.Tpid, realm sp.Trealm, ptype proc.Ttype) error {
	var res proto.AssignUprocdToRealmResponse
	req := &proto.AssignUprocdToRealmRequest{
		PidStr:      pid.String(),
		RealmStr:    realm.String(),
		ProcTypeInt: int64(ptype),
	}
	return kc.rpcc.RPC("KernelSrv.AssignUprocdToRealm", req, &res)
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

func (kc *KernelClnt) Port(pid sp.Tpid, p port.Tport) (string, port.PortBinding, error) {
	var res proto.PortResult
	req := &proto.PortRequest{PidStr: pid.String(), Port: int32(p)}
	if err := kc.rpcc.RPC("KernelSrv.AllocPort", req, &res); err != nil {
		return "", port.PortBinding{}, err
	}
	return res.HostIp, port.PortBinding{port.Tport(res.RealmPort), port.Tport(res.HostPort)}, nil
}
