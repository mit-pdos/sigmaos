package cgroup

import (
	"os"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

var cgroupsV2Checked bool

// Check if cgroupsv2, which SigmaOS depends on, are enabled.
func checkCgroupsV2() {
	if cgroupsV2Checked {
		return
	}
	s := time.Now()
	// If cgroupsv2 are enabled, the cgroup.controllers file will exist. This
	// could be present at /sys/fs/cgroup/cgroup.controllers (if the test program
	// imported this package), or otherwise /cgroup/cgroup.controllers (if a
	// container imported this package).
	_, err1 := os.Stat("/cgroup/cgroup.controllers")
	_, err2 := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	if err1 != nil && err2 != nil {
		db.DFatalf("Error stat cgroup.controllers: %v %v\n"+
			"\tMake sure cgroupsv2 are enabled: https://kubernetes.io/docs/concepts/architecture/cgroups/#linux-distribution-cgroup-v2-support",
			err1, err2,
		)
	}
	perf.LogSpawnLatency("DContainter check cgroupsv2", sp.NOT_SET, perf.TIME_NOT_SET, s)
	cgroupsV2Checked = true
}
