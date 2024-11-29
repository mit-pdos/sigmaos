package cgroup

import (
	"fmt"
)

type CPUStat struct {
	Util             float64
	totalSysUsecs    uint64
	totalCgroupUsecs uint64
}

func newCPUStat(totalSysUsecs, totalCgroupUsecs uint64, util float64) *CPUStat {
	return &CPUStat{
		Util:             util,
		totalSysUsecs:    totalSysUsecs,
		totalCgroupUsecs: totalCgroupUsecs,
	}
}

func (cst *CPUStat) String() string {
	return fmt.Sprintf("&{ util:%v }", cst.Util)
}
