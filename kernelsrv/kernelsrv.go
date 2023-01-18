package kernelsrv

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelsrv/proto"
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
	return pds.RunServer()
}

func (ks *KernelSrv) Boot(req proto.BootRequest, rep *proto.BootResult) error {
	if err := ks.k.BootSub(req.Name, ks.k.Param, false); err != nil {
		return err
	}
	return nil
}

func (ks *KernelSrv) Shutdown(req proto.ShutdownRequest, rep *proto.ShutdownResult) error {
	if err := ks.k.Shutdown(); err != nil {
		return err
	}
	os.Exit(0) // XXX use more elegant way
	return nil
}

func (ks *KernelSrv) Kill(req proto.KillRequest, rep *proto.KillResult) error {
	if err := ks.k.KillOne(req.Name); err != nil {
		return err
	}
	return nil
}
