package locald

import (
	"syscall"
	"unsafe"
)

const NCPU int = 1024

// CPUMask is a mask of cores passed to the Linux scheduler
type CPUMask struct {
	mask [(NCPU + 7) / 8]byte
}

// Returns true if the core is set in the mask.
func (m *CPUMask) Test(core int) bool {
	if core > NCPU {
		panic("core too high")
	}
	idx := core / 8
	bit := core % 8
	return m.mask[idx] & (1 << bit) != 0
}

// Sets a core in the mask.
func (m *CPUMask) Set(core int) {
	if core > NCPU {
		panic("core too high")
	}
	idx := core / 8
	bit := core % 8
	m.mask[idx] |= (1 << bit)
}

// Clears a core in the mask.
func (m *CPUMask) Clear(core int) {
	if core > NCPU {
		panic("core too high")
	}
	idx := core / 8
	bit := core % 8
	m.mask[idx] &= ^(1 << bit)
}

// Clears all cores in the mask.
func (m *CPUMask) ClearAll() {
	for i := range m.mask {
		m.mask[i] = 0
	}
}

// SchedSetAffinity pins a task to a mask of cores.
func SchedSetAffinity(pid int, m *CPUMask) error {
	_, _, errno := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY,
		uintptr(pid), uintptr(NCPU), uintptr(unsafe.Pointer(&m.mask)))
	if errno != 0 {
		return errno
	}
	return nil
}
