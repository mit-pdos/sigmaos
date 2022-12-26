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
	switch req.Name {
	case sp.PROCDREL:
		if err := ks.k.BootProcd(); err != nil {
			return err
		}
		rep.Ok = true
	case sp.S3REL:
		if err := ks.k.BootFss3d(); err != nil {
			return err
		}
		rep.Ok = true
	case sp.UXREL:
		if err := ks.k.BootFsUxd(); err != nil {
			return err
		}
		rep.Ok = true
	default:
		rep.Ok = false
	}
	return nil
}

func (ks *KernelSrv) Kill(req KillRequest, rep *KillResult) error {
	if err := ks.k.KillOne(req.Name); err != nil {
		return err
	}
	rep.Ok = true
	return nil
}
