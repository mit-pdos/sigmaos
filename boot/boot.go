package boot

import (
	"log"
	"os"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelsrv"
)

type Boot struct {
	k *kernel.Kernel
}

// The boot processes enters here
func BootUp(param *kernel.Param, nameds []string) (*Boot, error) {
	db.DPrintf(db.KERNEL, "Boot param %v nameds %v env %v", param, nameds, os.Environ())
	k, err := kernel.MakeKernel(param, nameds)
	if err != nil {
		return nil, err
	}

	log.Printf("container %s booted %v\n", os.Args[1], k.Ip())

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
