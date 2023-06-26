package cgroup

import (
	"path"
	"sync"

	db "sigmaos/debug"
)

const (
	SYS_CPU_STAT = "/proc/stat"
)

// Monitors statistics about cgroups.
type CgroupMonitor struct {
	sync.Mutex
	cfs *cgroupFs
}

func NewCgroupMonitor() *CgroupMonitor {
	return &CgroupMonitor{
		cfs: newCgroupFs(),
	}
}

func (cmon *CgroupMonitor) GetCPUShares(cgroupPath string) (int64, error) {
	cmon.Lock()
	defer cmon.Unlock()

	n, err := cmon.cfs.readFile(path.Join(cgroupPath, "cpu.weight"), parseUint64)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error readFile: %v", err)
		return 0, err
	}
	return int64(n), nil
}

func (cmon *CgroupMonitor) GetCPUUsecs(cgroupPath string) (uint64, error) {
	n, err := cmon.cfs.readFile(path.Join(cgroupPath, "cpu.stat"), parseCgroupCpuStat)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error readFile: %v", err)
		return 0, err
	}
	return n, nil
}

func (cmon *CgroupMonitor) GetSystemCPUUsecs() uint64 {
	n, err := cmon.cfs.readFile(SYS_CPU_STAT, cmon.cfs.parseSysCpuStat)
	if err != nil {
		db.DFatalf("Error readFile: %v", err)
	}
	return n
}
