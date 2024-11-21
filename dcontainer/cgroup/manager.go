package cgroup

import (
	"fmt"
	"path/filepath"
	"sync"

	db "sigmaos/debug"
)

// Manages cgroups. CgroupMonitor is a subset of CgroupMgr.
type CgroupMgr struct {
	sync.Mutex
	*CgroupMonitor
	cfs *cgroupFs
}

func NewCgroupMgr() *CgroupMgr {
	return &CgroupMgr{
		CgroupMonitor: NewCgroupMonitor(),
		cfs:           newCgroupFs(),
	}
}

func (cmgr *CgroupMgr) SetCPUShares(cgroupPath string, n int64) error {
	cmgr.Lock()
	defer cmgr.Unlock()

	if err := cmgr.cfs.writeFile(filepath.Join(cgroupPath, "cpu.weight"), uint64(n)); err != nil {
		db.DPrintf(db.ERROR, "Error writeFile: %v", err)
		return fmt.Errorf("Error writeFile: %v", err)
	}
	return nil
}

func (cmgr *CgroupMgr) SetMemoryLimit(cgroupPath string, membytes int64, memswap int64) error {
	cmgr.Lock()
	defer cmgr.Unlock()

	ps := []string{
		filepath.Join(cgroupPath, "memory.max"),
		filepath.Join(cgroupPath, "memory.swap.max"),
	}
	vals := []int64{
		membytes,
		memswap,
	}
	for i := range ps {
		p := ps[i]
		val := vals[i]
		err := cmgr.cfs.writeFile(p, uint64(val))
		if err != nil {
			if i == 1 {
				// Sometimes, on systems that have never had swap on, the
				// memory.max.swap file is missing. Ignore these errors, and print a
				// warning.
				db.DPrintf(db.ALWAYS, "Error getFD swap [%v]: %v", p, err)
				continue
			} else {
				// The memory.max file should always be present.
				db.DPrintf(db.ERROR, "Error getFD: %v", err)
				return fmt.Errorf("Error getFD: %v", err)
			}
		}
	}
	return nil
}
