package kernelclnt

import (
	"sigmaos/container"
	"sigmaos/fslib"
	"sigmaos/kernelsrv/proto"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
)

type KernelClnt struct {
	fsl *fslib.FsLib
	pdc *protdevclnt.ProtDevClnt
}

func MakeKernelClnt(fsl *fslib.FsLib, pn string) (*KernelClnt, error) {
	pdc, err := protdevclnt.MkProtDevClnt(fsl, pn)
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

func (kc *KernelClnt) Port(pid proc.Tpid, port string) (string, container.PortBinding, error) {
	var res proto.PortResult
	req := &proto.PortRequest{PidStr: pid.String(), Port: port}
	if err := kc.pdc.RPC("KernelSrv.Port", req, &res); err != nil {
		return "", container.PortBinding{}, err
	}
	return res.HostIp, container.PortBinding{res.RealmPort, res.HostPort}, nil
}
