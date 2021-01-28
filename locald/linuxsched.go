package locald

import (
	"bufio"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// NCPU is the maximum number of cores supported.
const NCPU uint = 1024
const bitsPerWord uint = uint(unsafe.Sizeof(uint(0)) * 8)

// CPUMask is a mask of cores passed to the Linux scheduler.
type CPUMask struct {
	mask [(NCPU + 7) / bitsPerWord]uint
}

// Test returns true if the core is set in the mask.
func (m *CPUMask) Test(core uint) bool {
	if core >= NCPU {
		panic("core too high")
	}
	idx := core / bitsPerWord
	bit := core % bitsPerWord
	return m.mask[idx]&(1<<bit) != 0
}

// Set sets a core in the mask.
func (m *CPUMask) Set(core uint) {
	if core >= NCPU {
		panic("core too high")
	}
	idx := core / bitsPerWord
	bit := core % bitsPerWord
	m.mask[idx] |= (1 << bit)
}

// Clear clears a core in the mask.
func (m *CPUMask) Clear(core uint) {
	if core >= NCPU {
		panic("core too high")
	}
	idx := core / bitsPerWord
	bit := core % bitsPerWord
	m.mask[idx] &= ^(1 << bit)
}

// OnesCount returns the number of one bits.
func (m *CPUMask) OnesCount() int {
	cnt := 0
	for i := range m.mask {
		cnt += bits.OnesCount(m.mask[i])
	}
	return cnt
}

// ClearAll clears all cores in the mask.
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

//const SYSFS_CPU_TOPOLOGY_PATH string = "/sys/devices/system/cpu/cpu%d/topology"

func sysfsParseVal(path string) (int, error) {
	// open the file
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// read the first line
	rd := bufio.NewReader(f)
	line, err := rd.ReadString('\n')
	if err != nil {
		return 0, err
	}

	// convert line to value
	line = strings.TrimSuffix(line, "\n")
	v, err := strconv.Atoi(line)
	if err != nil {
		return 0, err
	}

	return v, nil
}
