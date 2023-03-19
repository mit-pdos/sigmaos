package boot

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelsrv"
	sp "sigmaos/sigmap"
)

type Boot struct {
	k *kernel.Kernel
}

// The boot processes enters here
func BootUp(param *kernel.Param, nameds sp.Taddrs) error {
	db.DPrintf(db.KERNEL, "Boot param %v nameds %v env %v", param, nameds, os.Environ())
	k, err := kernel.MakeKernel(param, nameds)
	if err != nil {
		return err
	}

	db.DPrintf(db.BOOT, "container %s booted %v\n", os.Args[1], k.Ip())

	if err := kernelsrv.RunKernelSrv(k); err != nil {
		return err
	}
	return nil
}
