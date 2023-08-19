package boot

import (
	"os"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelsrv"
)

type Boot struct {
	k *kernel.Kernel
}

// The boot processes enters here
func BootUp(param *kernel.Param, scfg *config.SigmaConfig) error {
	db.DPrintf(db.KERNEL, "Boot param %v SigmaConfig %v env %v", param, scfg, os.Environ())
	k, err := kernel.MakeKernel(param, scfg)
	if err != nil {
		return err
	}

	db.DPrintf(db.BOOT, "container %s booted %v\n", os.Args[1], k.Ip())

	if err := kernelsrv.RunKernelSrv(k); err != nil {
		return err
	}

	return nil
}
