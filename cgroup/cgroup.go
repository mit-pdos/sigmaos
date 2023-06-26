package cgroup

import (
	"os"

	db "sigmaos/debug"
)

// Check if cgroupsv2, which SigmaOS depends on, are enabled.
func init() {
	// If cgroupsv2 are enabled, the cgroup.controllers file will exist.
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		db.DFatalf("Error stat cgroup.controllers: %v\n"+
			"\tMake sure cgroupsv2 are enabled: https://kubernetes.io/docs/concepts/architecture/cgroups/#linux-distribution-cgroup-v2-support",
			err,
		)
	}
}
