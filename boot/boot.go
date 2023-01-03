package boot

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelsrv"
)

type Boot struct {
	k *kernel.Kernel
}

// The boot processes enters here
func BootUp(param *kernel.Param) (*Boot, error) {
	db.DPrintf(db.KERNEL, "Boot param %v env %v\n", param, os.Environ())
	k, err := kernel.MakeKernel(param)
	if err != nil {
		return nil, err
	}
	if err := kernelsrv.RunKernelSrv(k); err != nil {
		return nil, err
	}
	return &Boot{k}, nil
}

func (b *Boot) Ip() string {
	return b.k.Ip()
}

func (b *Boot) ShutDown() error {
	return b.k.ShutDown()
}
