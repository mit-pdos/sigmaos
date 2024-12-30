package init

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernel/srv"
	"sigmaos/proc"
)

// The boot processes enters here
func BootUp(param *kernel.Param, pe *proc.ProcEnv) error {
	db.DPrintf(db.KERNEL, "Boot param %v ProcEnv %v env %v", param, pe, os.Environ())
	k, err := kernel.NewKernel(param, pe)
	if err != nil {
		return err
	}
	db.DPrintf(db.BOOT, "container %s booted %v\n", os.Args[1], k.Ip())
	if err := srv.Run(k); err != nil {
		return err
	}
	return nil
}
