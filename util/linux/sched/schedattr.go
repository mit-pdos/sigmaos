package sched

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

/*
#include "linux/sched/types.h"
*/
import "C"

type SchedPolicy uint32
type SchedFlag uint64
type SchedDeadline uint64

const (
	SCHED_NORMAL   SchedPolicy = 0
	SCHED_FIFO     SchedPolicy = 1
	SCHED_RR       SchedPolicy = 2
	SCHED_BATCH    SchedPolicy = 3
	SCHED_IDLE     SchedPolicy = 5
	SCHED_DEADLINE SchedPolicy = 6

	ResetOnFork SchedFlag = 1
)

type SchedAttr struct {
	Size         uint32
	Policy       SchedPolicy
	Flags        SchedFlag
	Nice         int32
	Priority     uint32
	Runtime      SchedDeadline
	Deadline     SchedDeadline
	Period       SchedDeadline
	SchedUtilMin uint32
	SchedUtilMax uint32
}

func SchedGetAttr(pid int) (*SchedAttr, error) {
	attr := C.struct_sched_attr{}
	_, _, errno := unix.Syscall6(unix.SYS_SCHED_GETATTR, uintptr(pid),
		uintptr(unsafe.Pointer(&attr)), uintptr(C.SCHED_ATTR_SIZE_VER0), uintptr(0), uintptr(0), uintptr(0))
	if errno != 0 {
		return nil, errno
	}
	return &SchedAttr{
		Size:         uint32(attr.size),
		Policy:       SchedPolicy(attr.sched_policy),
		Flags:        SchedFlag(attr.sched_flags),
		Nice:         int32(attr.sched_nice),
		Priority:     uint32(attr.sched_priority),
		Runtime:      SchedDeadline(attr.sched_runtime),
		Deadline:     SchedDeadline(attr.sched_deadline),
		Period:       SchedDeadline(attr.sched_period),
		SchedUtilMin: uint32(attr.sched_util_min),
		SchedUtilMax: uint32(attr.sched_util_max),
	}, nil
}

func SchedSetAttr(pid int, attr *SchedAttr) error {
	cattr := C.struct_sched_attr{
		size:           C.__u32(C.SCHED_ATTR_SIZE_VER0),
		sched_policy:   C.__u32(attr.Policy),
		sched_flags:    C.__u64(attr.Flags),
		sched_nice:     C.__s32(attr.Nice),
		sched_priority: C.__u32(attr.Priority),
		sched_runtime:  C.__u64(attr.Runtime),
		sched_deadline: C.__u64(attr.Deadline),
		sched_period:   C.__u64(attr.Period),
		sched_util_min: C.__u32(attr.SchedUtilMin),
		sched_util_max: C.__u32(attr.SchedUtilMax),
	}
	_, _, errno := unix.Syscall(unix.SYS_SCHED_SETATTR, uintptr(pid),
		uintptr(unsafe.Pointer(&cattr)), uintptr(0))
	if errno != 0 {
		return errno
	}
	return nil
}
