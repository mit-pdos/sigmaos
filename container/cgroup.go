package container

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	db "sigmaos/debug"
)

const (
	// The value comes from `C.sysconf(C._SC_CLK_TCK)`, and
	// on Linux it's a constant which is safe to be hard coded,
	// so we can avoid using cgo here. For details, see:
	// https://github.com/containerd/cgroups/pull/12
	clockTicksPerSecond   = 100
	microSecondsPerSecond = 1e6
)

func (c *Container) getCPUShares() int64 {
	p := path.Join(c.cgroupPath, "cpu.weight")
	f, err := os.Open(p)
	if err != nil {
		db.DFatalf("Error open: %v", err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		db.DFatalf("Error read: %v", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	err = f.Close()
	if err != nil {
		db.DFatalf("Error close: %v", err)
	}
	return n
}

func (c *Container) setCPUShares(n int64) {
	p := path.Join(c.cgroupPath, "cpu.weight")
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		db.DFatalf("Error open: %v", err)
	}
	b := []byte(strconv.FormatInt(n, 10))
	n2, err := f.Write(b)
	if err != nil || n2 != len(b) {
		db.DPrintf(db.ALWAYS, "Error write: %v n: %v", err, n2)
		return
	}
	err = f.Close()
	if err != nil {
		db.DFatalf("Error close: %v", err)
	}
}

func (c *Container) setMemoryLimit(membytes int64, memswap int64) {
	ps := []string{
		path.Join(c.cgroupPath, "memory.max"),
		path.Join(c.cgroupPath, "memory.swap.max"),
	}
	vals := []int64{
		membytes,
		memswap,
	}
	for i := range ps {
		p := ps[i]
		m := vals[i]
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			db.DFatalf("Error open: %v", err)
		}
		b := []byte(strconv.FormatInt(m, 10))
		n, err := f.Write(b)
		if err != nil || n != len(b) {
			db.DPrintf(db.ALWAYS, "Error write: %v n: %v", err, n)
			return
		}
		err = f.Close()
		if err != nil {
			db.DFatalf("Error close: %v", err)
		}
	}
}

func (c *Container) getContainerCPUUsecs() uint64 {
	if c.cpustatf == nil {
		var err error
		c.cpustatf, err = os.Open(path.Join(c.cgroupPath, "cpu.stat"))
		if err != nil {
			db.DFatalf("Couldn't open cpu stat file: %v", err)
		}
	} else {
		off, err := c.cpustatf.Seek(0, 0)
		if err != nil || off != 0 {
			db.DFatalf("Error seeking in file: off %v err %v", off, err)
		}
	}
	b, err := ioutil.ReadAll(c.cpustatf)
	if err != nil {
		db.DFatalf("Couldn't read cpu stat file: %v", err)
	}
	totalUsecsStr := strings.Fields(string(b))[1]
	totalUsecs, err := strconv.ParseUint(totalUsecsStr, 10, 64)
	if err != nil {
		db.DFatalf("Error strconv totalUsecs: %v", err)
	}
	return totalUsecs
}

// Based on Docker's implementation:
// https://github.com/moby/moby/blob/master/daemon/stats/collector_unix.go
//
// getSystemCPUUsage returns the host system's cpu usage in
// microseconds. Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func (c *Container) getSystemCPUUsecs() uint64 {
	if c.sysstatf == nil {
		var err error
		c.sysstatf, err = os.Open("/proc/stat")
		if err != nil {
			db.DFatalf("Couldn't open sys stat file: %v", err)
		}
		c.br = bufio.NewReader(c.sysstatf)
	} else {
		off, err := c.sysstatf.Seek(0, 0)
		if err != nil || off != 0 {
			db.DFatalf("Error seeking in file: off %v err %v", off, err)
		}
		c.br.Reset(c.sysstatf)
	}

	defer c.br.Reset(nil)

	for {
		line, err := c.br.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				db.DFatalf("invalid number of cpu fields %v", parts)
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					db.DFatalf("Unable to convert value %s to int: %s", i, err)
				}
				totalClockTicks += v
			}
			return (totalClockTicks * microSecondsPerSecond) / clockTicksPerSecond
		}
	}
	db.DFatalf("error getSysCPUUsage")
	return 0
}
