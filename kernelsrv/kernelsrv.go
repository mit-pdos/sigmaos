package kernelsrv

import (
	"os"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernel"
	"sigmaos/kernelsrv/proto"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
)

type KernelSrv struct {
	k  *kernel.Kernel
	ch chan struct{}
}

func RunKernelSrv(k *kernel.Kernel) error {
	ks := &KernelSrv{k: k}
	ks.ch = make(chan struct{})
	db.DPrintf(db.KERNEL, "%v: Run KernelSrv %v", proc.GetName(), k.Param.KernelId)
	_, err := protdevsrv.MakeProtDevSrvClnt(sp.BOOT+k.Param.KernelId, k.SigmaClnt, ks)
	if err != nil {
		return err
	}
	// let start-kernel.sh know that the kernel is ready
	f, err := os.Create("/tmp/sigmaos/" + k.Param.KernelId)
	if err != nil {
		return err
	}
	f.Close()
	<-ks.ch
	db.DPrintf(db.KERNEL, "%v: Run KernelSrv done %v", proc.GetName(), k.Param.KernelId)
	return nil
}

func (ks *KernelSrv) Boot(ctx fs.CtxI, req proto.BootRequest, rep *proto.BootResult) error {
	var pid proc.Tpid
	var err error
	if pid, err = ks.k.BootSub(req.Name, req.Args, ks.k.Param, false); err != nil {
		return err
	}
	rep.PidStr = pid.String()
	return nil
}

func (ks *KernelSrv) SetCPUShares(ctx fs.CtxI, req proto.SetCPUSharesRequest, rep *proto.SetCPUSharesResponse) error {
	return ks.k.SetCPUShares(proc.Tpid(req.PidStr), req.Shares)
}

func (ks *KernelSrv) GetCPUUtil(ctx fs.CtxI, req proto.GetKernelSrvCPUUtilRequest, rep *proto.GetKernelSrvCPUUtilResponse) error {
	util, err := ks.k.GetCPUUtil(proc.Tpid(req.PidStr))
	if err != nil {
		return err
	}
	rep.Util = util
	return nil
}

func (ks *KernelSrv) Shutdown(ctx fs.CtxI, req proto.ShutdownRequest, rep *proto.ShutdownResult) error {
	if err := ks.k.Shutdown(); err != nil {
		return err
	}
	ks.ch <- struct{}{}
	return nil
}

func (ks *KernelSrv) Kill(ctx fs.CtxI, req proto.KillRequest, rep *proto.KillResult) error {
	return ks.k.KillOne(req.Name)
}

func (ks *KernelSrv) AllocPort(ctx fs.CtxI, req proto.PortRequest, rep *proto.PortResult) error {
	pb, err := ks.k.AllocPort(proc.Tpid(req.PidStr), port.Tport(req.Port))
	if err != nil {
		return err
	}
	ip, err := container.LocalIP()
	if err != nil {
		return err
	}

	rep.RealmPort = int32(pb.RealmPort)
	rep.HostPort = int32(pb.HostPort)
	rep.HostIp = ip
	return nil
}
