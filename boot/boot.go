package kernel

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
func BootUp(realm, pn string) (*Boot, error) {
	db.DPrintf(db.KERNEL, "Boot %s %s\n", realm, pn)

	param, err := kernel.ReadParam(pn)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.KERNEL, "Boot %s param %v env %v\n", pn, param, os.Environ())
	k, err := kernel.MakeKernel(realm, param)
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
