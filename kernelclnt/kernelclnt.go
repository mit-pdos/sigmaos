package kernelclnt

import (
	"sigmaos/fslib"
	"sigmaos/kernelsrv/proto"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
)

type KernelClnt struct {
	fsl *fslib.FsLib
	pdc *rpcclnt.RPCClnt
}

func MakeKernelClnt(fsl *fslib.FsLib, pn string) (*KernelClnt, error) {
	pdc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{fsl}, pn)
	if err != nil {
		return nil, err
	}
	return &KernelClnt{fsl, pdc}, nil
}

func (kc *KernelClnt) Boot(s string, args []string) (proc.Tpid, error) {
	var res proto.BootResult
	req := &proto.BootRequest{Name: s, Args: args}
	err := kc.pdc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return proc.Tpid(""), err
	}
	return proc.Tpid(res.PidStr), nil
}

func (kc *KernelClnt) SetCPUShares(pid proc.Tpid, shares int64) error {
	var res proto.SetCPUSharesResponse
	req := &proto.SetCPUSharesRequest{PidStr: pid.String(), Shares: shares}
	return kc.pdc.RPC("KernelSrv.SetCPUShares", req, &res)
}

func (kc *KernelClnt) GetCPUUtil(pid proc.Tpid) (float64, error) {
	var res proto.GetKernelSrvCPUUtilResponse
	req := &proto.GetKernelSrvCPUUtilRequest{PidStr: pid.String()}
	err := kc.pdc.RPC("KernelSrv.GetCPUUtil", req, &res)
	if err != nil {
		return 0.0, err
	}
	return res.Util, nil
}

func (kc *KernelClnt) Kill(s string) error {
	var res proto.KillResult
	req := &proto.KillRequest{Name: s}
	return kc.pdc.RPC("KernelSrv.Kill", req, &res)
}

func (kc *KernelClnt) Shutdown() error {
	var res proto.ShutdownResult
	req := &proto.ShutdownRequest{}
	return kc.pdc.RPC("KernelSrv.Shutdown", req, &res)
}

func (kc *KernelClnt) Port(pid proc.Tpid, p port.Tport) (string, port.PortBinding, error) {
	var res proto.PortResult
	req := &proto.PortRequest{PidStr: pid.String(), Port: int32(p)}
	if err := kc.pdc.RPC("KernelSrv.AllocPort", req, &res); err != nil {
		return "", port.PortBinding{}, err
	}
	return res.HostIp, port.PortBinding{port.Tport(res.RealmPort), port.Tport(res.HostPort)}, nil
}
