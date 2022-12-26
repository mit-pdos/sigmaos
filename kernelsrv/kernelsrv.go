package kernelsrv

import (
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
)

type KernelSrv struct {
	s *kernel.System
}

func RunKernelSrv(s *kernel.System) error {
	ks := &KernelSrv{s}
	db.DPrintf(db.KERNEL, "%v: Run KernelSrv", proc.GetName())
	pds, err := protdevsrv.MakeProtDevSrvPriv(sp.BOOT, s.FsLib, ks)
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
	case "procd":
		if err := ks.s.BootProcd(); err != nil {
			return err
		}
		rep.Ok = true
	case "fss3d":
		if err := ks.s.BootFss3d(); err != nil {
			return err
		}
		rep.Ok = true
	default:
		rep.Ok = false
	}
	return nil
}

func (ks *KernelSrv) Kill(req KillRequest, rep *KillResult) error {
	if err := ks.s.KillOne(req.Name); err != nil {
		return err
	}
	rep.Ok = true
	return nil
}
