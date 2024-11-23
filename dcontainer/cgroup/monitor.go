package cgroup

import (
	"fmt"
	"path/filepath"
	"sync"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
)

const (
	SYS_CPU_STAT = "/proc/stat"
)

// Monitors statistics about cgroups.
type CgroupMonitor struct {
	sync.Mutex
	cfs     *cgroupFs
	cpuStat map[string]*CPUStat
}

func NewCgroupMonitor() *CgroupMonitor {
	return &CgroupMonitor{
		cfs:     newCgroupFs(),
		cpuStat: make(map[string]*CPUStat),
	}
}

func (cmon *CgroupMonitor) GetCPUStats(cgroupPath string) (*CPUStat, error) {
	cmon.Lock()
	defer cmon.Unlock()

	var prev *CPUStat
	var ok bool
	if prev, ok = cmon.cpuStat[cgroupPath]; !ok {
		prev = newCPUStat(0, 0, 0.0)
		cmon.cpuStat[cgroupPath] = prev
	}

	var util float64 = prev.Util
	// Read total CPU time for cgroup.
	u1, err := cmon.getCPUUsecs(cgroupPath)
	if err != nil {
		db.DPrintf(db.ERROR, "Error get CPU usecs container: %v", err)
		return nil, fmt.Errorf("Error get CPU usecs container: %v", err)
	}
	// Read total CPU time for the entire system
	s1, err := cmon.getSystemCPUUsecs()
	if err != nil {
		return nil, err
	}

	// If this isn't the first time GetCPUStats has been called, calculate
	// utilization.
	if prev.totalSysUsecs > 0 {
		ctrDelta := u1 - prev.totalCgroupUsecs
		sysDelta := s1 - prev.totalSysUsecs
		// CPU util calculation based on
		// https://github.com/moby/moby/blob/eb131c5383db8cac633919f82abad86c99bffbe5/cli/command/container/stats_helpers.go#L175
		if sysDelta > 0 && ctrDelta > 0 {
			util = float64(ctrDelta) / float64(sysDelta) * float64(linuxsched.GetNCores()) * 100.0
		} else {
			db.DPrintf(db.ALWAYS, "GetCPUUtil [%v] no delta %v %v", cgroupPath, sysDelta, ctrDelta)
		}
		db.DPrintf(db.CGROUP, "GetCPUUtil ctrDelta %v sysDelta %v util %v", ctrDelta, sysDelta, util)
	}

	cur := newCPUStat(s1, u1, util)
	// Update saved cpu stats.
	*cmon.cpuStat[cgroupPath] = *cur

	return cur, nil
}

func (cmon *CgroupMonitor) GetPIDs(cgroupPath string) ([]int, error) {
	cmon.Lock()
	defer cmon.Unlock()

	pids, err := cmon.cfs.readFileMulti(filepath.Join(cgroupPath, "cgroup.procs"), parseInts)
	if err != nil {
		db.DPrintf(db.ERROR, "Error readFile: %v", err)
		return nil, fmt.Errorf("Error readFile: %v", err)
	}
	return pids, nil
}

func (cmon *CgroupMonitor) getCPUShares(cgroupPath string) (int64, error) {

	n, err := cmon.cfs.readFile(filepath.Join(cgroupPath, "cpu.weight"), parseUint64)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error readFile: %v", err)
		return 0, err
	}
	return int64(n), nil
}

func (cmon *CgroupMonitor) getCPUUsecs(cgroupPath string) (uint64, error) {
	n, err := cmon.cfs.readFile(filepath.Join(cgroupPath, "cpu.stat"), parseCgroupCpuStat)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error readFile: %v", err)
		return 0, err
	}
	return n, nil
}

func (cmon *CgroupMonitor) getSystemCPUUsecs() (uint64, error) {
	n, err := cmon.cfs.readFile(SYS_CPU_STAT, cmon.cfs.parseSysCpuStat)
	if err != nil {
		db.DPrintf(db.ERROR, "Error readFile: %v", err)
		return 0, fmt.Errorf("Error readFile: %v", err)
	}
	return n, nil
}
