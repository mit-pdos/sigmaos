package kernelsrv

import (
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
)

type KernelSrv struct {
	k *kernel.Kernel
}

func RunKernelSrv(k *kernel.Kernel) error {
	ks := &KernelSrv{k}
	db.DPrintf(db.KERNEL, "%v: Run KernelSrv", proc.GetName())
	pds, err := protdevsrv.MakeProtDevSrvPriv(sp.BOOT, k.FsLib, ks)
	if err != nil {
		return err
	}
	go func() {
		pds.RunServer()
	}()
	return nil
}

func (ks *KernelSrv) Boot(req BootRequest, rep *BootResult) error {
	if err := ks.k.BootSub(req.Name); err != nil {
		return err
	}
	rep.Ok = true
	return nil
}

func (ks *KernelSrv) Kill(req KillRequest, rep *KillResult) error {
	if err := ks.k.KillOne(req.Name); err != nil {
		return err
	}
	rep.Ok = true
	return nil
}
