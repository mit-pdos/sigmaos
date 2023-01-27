package kernelsrv

import (
	db "sigmaos/debug"
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
	pds, err := protdevsrv.MakeProtDevSrvPriv(sp.BOOT, k.SigmaClnt.FsLib, ks)
	if err != nil {
		return err
	}
	go pds.RunServer()
	<-ks.ch
	pds.Done()
	return nil
}

func (ks *KernelSrv) Boot(req proto.BootRequest, rep *proto.BootResult) error {
	var pid proc.Tpid
	var err error
	if pid, err = ks.k.BootSub(req.Name, req.Args, ks.k.Param, false); err != nil {
		return err
	}
	rep.PidStr = pid.String()
	return nil
}

func (ks *KernelSrv) SetCPUShares(req proto.SetCPUSharesRequest, rep *proto.SetCPUSharesResponse) error {
	if err := ks.k.SetCPUShares(proc.Tpid(req.PidStr), req.Shares); err != nil {
		return err
	}
	return nil
}

func (ks *KernelSrv) Shutdown(req proto.ShutdownRequest, rep *proto.ShutdownResult) error {
	if err := ks.k.Shutdown(); err != nil {
		return err
	}
	ks.ch <- struct{}{}
	return nil
}

func (ks *KernelSrv) Kill(req proto.KillRequest, rep *proto.KillResult) error {
	if err := ks.k.KillOne(req.Name); err != nil {
		return err
	}
	return nil
}
