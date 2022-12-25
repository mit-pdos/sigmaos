package kernel

import (
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelsrv"
)

type Boot struct {
	s *kernel.System
}

// The boot processes enters here
func BootUp(pn string) (*Boot, error) {
	db.DPrintf(db.KERNEL, "Boot %s\n", pn)
	param, err := kernel.ReadParam(pn)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.KERNEL, "Boot %s param %v\n", pn, param)
	s, err := kernel.MakeKernel(param)
	if err != nil {
		return nil, err
	}
	if err := kernelsrv.RunKernelSrv(s); err != nil {
		return nil, err
	}
	return &Boot{s}, nil
}

func (b *Boot) ShutDown() error {
	return b.s.ShutDown()
}
