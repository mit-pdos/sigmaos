package kernelsrv

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/system"
)

type KernelSrv struct {
	s *system.System
}

func RunKernelSrv(s *system.System) error {
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
