package locald

import (
	"bufio"
	"errors"
	"fmt"
	"math/bits"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var ErrInvalid = errors.New("invalid")

// NCPU is the maximum number of cores supported.
const NCPU uint = 1024
const bitsPerWord uint = uint(unsafe.Sizeof(uint(0)) * 8)

// CPUMask is a mask of cores passed to the Linux scheduler.
type CPUMask struct {
	mask [(NCPU + bitsPerWord - 1) / bitsPerWord]uint
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

// CreateCPUMaskOfOne creates a mask of one core.
func CreateCPUMaskOfOne(core uint) *CPUMask {
	mask := new(CPUMask)
	mask.Set(core)
	return mask
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

func fsReadInt(path string) (int, error) {
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

func fsWriteInt(path string, val int) error {
	// open the file for writing
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// convert value to string
	s := strconv.Itoa(val)

	// write the string to the file
	if _, err := f.Write([]byte(s)); err != nil {
		return err
	}

	return nil
}

func fsReadString(path string) (string, error) {
	// open the file
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// read the first line
	rd := bufio.NewReader(f)
	line, err := rd.ReadString('\n')
	if err != nil {
		return "", err
	}

	// convert line to string
	line = strings.TrimSuffix(line, "\n")
	return line, nil
}

func fsReadBitlist(path string) (*CPUMask, error) {
	// open the file
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// read the first line
	rd := bufio.NewReader(f)
	line, err := rd.ReadString('\n')
	if err != nil {
		return nil, err
	}

	// convert line to mask
	mask := new(CPUMask)
	line = strings.TrimSuffix(line, "\n")
	sarr := strings.Split(line, ",")
	for _, s := range sarr {
		v, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		if v < 0 {
			return nil, fmt.Errorf("%v: %w", line, ErrInvalid)
		}
		mask.Set(uint(v))
	}

	return mask, nil
}

func fsWriteBitlist(path string, mask *CPUMask) error {
	// open the file for writing
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// convert mask to string
	var sb strings.Builder
	for i := uint(0); i < NCPU; i++ {
		if mask.Test(i) {
			sb.WriteString(strconv.Itoa(int(i)) + ",")
		}
	}
	s := strings.TrimSuffix(sb.String(), ",")

	// write the string to the file
	if _, err := f.Write([]byte(s)); err != nil {
		return err
	}

	return nil
}

func irqSetAffinity(irq int, mask *CPUMask) error {
	path := "/proc/irq/" + strconv.Itoa(irq) + "/smp_affinity_list"
	return fsWriteBitlist(path, mask)
}

func irqGetAffinity(irq int) (*CPUMask, error) {
	path := "/proc/irq/" + strconv.Itoa(irq) + "/smp_affinity_list"
	return fsReadBitlist(path)
}

func irqGetActions(irq int) (string, error) {
	path := "/sys/kernel/irq/" + strconv.Itoa(irq) + "/actions"
	return fsReadString(path)
}

func topologyPath(core int) string {
	return "/sys/devices/system/cpu/cpu" + strconv.Itoa(core) + "/topology"
}

func coreGetPackageSiblings(core int) (*CPUMask, error) {
	path := topologyPath(core) + "/package_cpus_list"
	return fsReadBitlist(path)
}

func coreGetThreadSiblings(core int) (*CPUMask, error) {
	path := topologyPath(core) + "/thread_siblings_list"
	return fsReadBitlist(path)
}

func coreGetPackageId(core int) (int, error) {
	path := topologyPath(core) + "/physical_package_id"
	return fsReadInt(path)
}

type PackageInfo struct {
	Node int
	ThreadSiblings []*CPUMask
	PackageSiblings *CPUMask
}

type TopologyInfo struct {
	Packages map[int]*PackageInfo
}

func scanTopologyOne(core int, ti *TopologyInfo) error {
	node, err := coreGetPackageId(core)
	if err != nil {
		return err
	}

	si := ti.Packages[node]
	if si == nil {
		si = new(PackageInfo)
		ti.Packages[node] = si
		si.Node = node
		si.PackageSiblings, err = coreGetPackageSiblings(core)
		if err != nil {
			return err
		}
	}

	for _, v := range si.ThreadSiblings {
		if v.Test(uint(core)) {
			return nil
		}
	}

	threadSiblings, err := coreGetThreadSiblings(core)
	if err != nil {
		return err
	}
	si.ThreadSiblings = append(si.ThreadSiblings, threadSiblings)
	return nil
}

func ScanTopology() (*TopologyInfo, error) {
	// open the sysfs cpu directory
	files, err := ioutil.ReadDir("/sys/devices/system/cpu/")
	if err != nil {
		return nil, err
	}

	// create topology info structure
	ti := new(TopologyInfo)
	ti.Packages = make(map[int]*PackageInfo)

	// scan each core in sysfs
	for _, f := range files {
		s := f.Name()
		if !strings.HasPrefix(s, "cpu") {
			continue
		}
		v, err := strconv.Atoi(strings.TrimPrefix(s, "cpu"))
		if err != nil {
			continue
		}

		err = scanTopologyOne(v, ti)
		if err != nil {
			return nil, err
		}
	}

	return ti, nil
}


