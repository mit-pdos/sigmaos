package linuxsched

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	db "sigmaos/debug"
	"sigmaos/proc"
	"time"
)

// NCores is the actual number of cores in the machine.
var nCores uint

func GetNCores() uint {
	if nCores == 0 {
		s := time.Now()
		if _, err := ScanTopology(); err != nil {
			db.DFatalf("ScanTopology failed %v", err)
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] Linuxsched scanTopology latency: %v", proc.GetSigmaDebugPid(), time.Since(s))
	}
	return nCores
}

var ErrInvalid = errors.New("invalid")

// MaxCores is the maximum number of cores supported.
const MaxCores uint = 256

const bitsPerWord uint = uint(unsafe.Sizeof(uint(0)) * 8)
const bitsPerUint32 uint = 32

// CPUMask is a mask of cores passed to the Linux scheduler.
type CPUMask struct {
	mask [(MaxCores + bitsPerWord - 1) / bitsPerWord]uint
}

// Test returns true if the core is set in the mask.
func (m *CPUMask) Test(core uint) bool {
	if core >= GetNCores() {
		panic("core too high")
	}
	idx := core / bitsPerWord
	bit := core % bitsPerWord
	return m.mask[idx]&(1<<bit) != 0
}

// Set sets a core in the mask.
func (m *CPUMask) Set(core uint) {
	if core >= GetNCores() {
		panic("core too high")
	}
	idx := core / bitsPerWord
	bit := core % bitsPerWord
	m.mask[idx] |= (1 << bit)
}

// Clear clears a core in the mask.
func (m *CPUMask) Clear(core uint) {
	if core >= GetNCores() {
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

// FindNextSet gets the index of the next set bit from a starting position.
func (m *CPUMask) FindNextSet(pos uint) uint {
	mask := ^uint((1 << (pos % bitsPerWord)) - 1)

	for idx := pos & ^uint(bitsPerWord-1); idx < GetNCores(); idx += bitsPerWord {
		val := m.mask[idx/bitsPerWord]
		val &= mask
		if val != 0 {
			return idx + uint(bits.TrailingZeros(val))
		}
		mask = 0
	}

	return GetNCores()
}

// ClearAll clears all cores in the mask.
func (m *CPUMask) ClearAll() {
	for i := range m.mask {
		m.mask[i] = 0
	}
}

func (m *CPUMask) getUInt32(idx int) uint32 {
	shift := uint(idx) * bitsPerUint32 % bitsPerWord
	val := m.mask[uint(idx)*bitsPerUint32/bitsPerWord]
	return uint32((val >> shift) & 0xFFFFFFFF)
}

func (m *CPUMask) setUInt32(idx int, val uint32) {
	shift := uint(idx) * bitsPerUint32 % bitsPerWord
	uidx := uint(idx) * bitsPerUint32 / bitsPerWord
	m.mask[uidx] &= ^(0xFFFFFFFF << shift)
	m.mask[uidx] |= uint(val) << shift
}

// CreateCPUMaskOfOne creates a mask of one core.
func CreateCPUMaskOfOne(core uint) *CPUMask {
	mask := new(CPUMask)
	mask.Set(core)
	return mask
}

func SchedSetPriority(pid int, prio int) error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, pid, prio)
}

// SchedSetAffinityAllTasks pins all of a process's tasks to a mask of cores.
func SchedSetAffinityAllTasks(procPid int, m *CPUMask) error {
	pids := []int{}
	taskDirPath := filepath.Join("/proc", strconv.Itoa(procPid), "task")
	ps, err := ioutil.ReadDir(taskDirPath)
	if err != nil {
		return fmt.Errorf("Error getting task pids: %v, %v", taskDirPath, err)
	}
	for _, p := range ps {
		pid, err := strconv.Atoi(p.Name())
		if err != nil {
			return fmt.Errorf("Error converting task pid to int: %v, %v", p.Name(), err)
		}
		pids = append(pids, pid)
	}
	for _, pid := range pids {
		err := SchedSetAffinity(pid, m)
		if err != nil {
			return fmt.Errorf("Error setting core affinity: %v", err)
		}
	}
	return nil
}

// SchedSetAffinity pins a task to a mask of cores.
func SchedSetAffinity(pid int, m *CPUMask) error {
	_, _, errno := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY,
		uintptr(pid), uintptr(len(m.mask)*8), uintptr(unsafe.Pointer(&m.mask)))
	if errno != 0 {
		return errno
	}
	return nil
}

// SchedSetAffinity pins a task to a mask of cores.
func SchedGetAffinity(pid int) (*CPUMask, error) {
	m := new(CPUMask)
	_, _, errno := syscall.Syscall(syscall.SYS_SCHED_GETAFFINITY,
		uintptr(pid), uintptr( /*len(m.mask)*8*/ 128), uintptr(unsafe.Pointer(&m.mask)))
	if errno != 0 {
		return nil, errno
	}
	return m, nil
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

func fsReadCPUMask(path string) (*CPUMask, error) {
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
	groups := strings.Split(line, ",")
	for i, group := range groups {
		v, err := strconv.ParseUint(group, 16, 32)
		if err != nil {
			return nil, err
		}
		mask.setUInt32(len(groups)-i-1, uint32(v))
	}

	return mask, nil
}

func fsWriteCPUMask(path string, mask *CPUMask) error {
	// open the file for writing
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// convert mask to string
	var sb strings.Builder
	for i := int(GetNCores() / 32); i >= 0; i-- {
		v := mask.getUInt32(i)
		sb.WriteString(fmt.Sprintf("%08x,", v))
	}
	s := strings.TrimSuffix(sb.String(), ",")

	// write the string to the file
	if _, err := f.Write([]byte(s)); err != nil {
		return err
	}

	return nil

}

func fsReadContiguousBitlist(path string) (int, error) {
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

	// convert to range and validate
	line = strings.TrimSuffix(line, "\n")
	groups := strings.Split(line, "-")
	if len(groups) == 2 {
		start, err1 := strconv.Atoi(groups[0])
		end, err2 := strconv.Atoi(groups[1])
		if err1 != nil || err2 != nil || start != 0 {
			return 0, ErrInvalid
		}
		return end + 1, nil
	}
	_, err = strconv.Atoi(line)
	return 1, err
}

func irqSetAffinity(irq int, mask *CPUMask) error {
	path := "/proc/irq/" + strconv.Itoa(irq) + "/smp_affinity"
	return fsWriteCPUMask(path, mask)
}

func irqGetAffinity(irq int) (*CPUMask, error) {
	path := "/proc/irq/" + strconv.Itoa(irq) + "/smp_affinity"
	return fsReadCPUMask(path)
}

func irqGetActions(irq int) (string, error) {
	path := "/sys/kernel/irq/" + strconv.Itoa(irq) + "/actions"
	return fsReadString(path)
}

func topologyPath(core int) string {
	return "/sys/devices/system/cpu/cpu" + strconv.Itoa(core) + "/topology"
}

func coreGetPackageSiblings(core int) (*CPUMask, error) {
	path := topologyPath(core) + "/package_cpus"
	return fsReadCPUMask(path)
}

func coreGetThreadSiblings(core int) (*CPUMask, error) {
	path := topologyPath(core) + "/thread_siblings"
	return fsReadCPUMask(path)
}

func coreGetPackageID(core int) (int, error) {
	path := topologyPath(core) + "/physical_package_id"
	return fsReadInt(path)
}

// PackageInfo is information about one physical package.
type PackageInfo struct {
	Node            int
	ThreadSiblings  []*CPUMask
	PackageSiblings *CPUMask
}

// TopologyInfo is information about CPU topology (packages, threads, etc.).
type TopologyInfo struct {
	Packages map[int]*PackageInfo
}

func scanCore(core int, ti *TopologyInfo) error {
	node, err := coreGetPackageID(core)
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

func coreIsOnline(cpu string) (bool, error) {
	// cpu0 is always online.
	if cpu == "cpu0" {
		return true, nil
	}
	if i, err := fsReadInt(filepath.Join("/sys/devices/system/cpu", cpu, "online")); err != nil {
		return false, err
	} else {
		return i == 1, nil
	}
}

// ScanTopology reports the topology of the machine (packages, threads, etc.)
func ScanTopology() (*TopologyInfo, error) {
	// read the number of online cores
	n, err := fsReadContiguousBitlist("/sys/devices/system/cpu/online")
	if err != nil {
		return nil, err
	}
	nCores = uint(n)

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
		online, err := coreIsOnline(s)
		if err != nil {
			return nil, err
		}
		if online {
			err = scanCore(v, ti)
			if err != nil {
				return nil, err
			}
		}
	}

	return ti, nil
}

// PrintTopology prints the topology of the machine to stdout.
func PrintTopology(ti *TopologyInfo) {
	for i, v := range ti.Packages {
		s := "node" + strconv.Itoa(i) + ": "
		for _, sibs := range v.ThreadSiblings {
			pos := uint(0)
			s += "["
			for {
				pos = sibs.FindNextSet(pos)
				if pos >= GetNCores() {
					break
				}
				s += strconv.Itoa(int(pos)) + ","
				pos++
			}
			s = strings.TrimSuffix(s, ",")
			s += "]"
		}
		fmt.Println(s)
	}
}
