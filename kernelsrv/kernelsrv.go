package kernelsrv

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernel"
	"sigmaos/kernelsrv/proto"
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
	db.DPrintf(db.KERNEL, "%v: Run KernelSrv", proc.GetName())
	pds, err := protdevsrv.MakeProtDevSrvPriv(sp.BOOT, k.SigmaClnt, ks)
	if err != nil {
		return err
	}
	go pds.RunServer()
	<-ks.ch
	pds.Done()
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
