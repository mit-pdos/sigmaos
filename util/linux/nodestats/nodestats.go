// Package nodestats periodically logs whole-node resource state, so that
// per-proc latency can be interpreted against what the node was doing at the
// time: was a proc slow because it did more work, or because it was waiting
// for a CPU?
//
// The key field is PSI (pressure stall information, /proc/pressure/*): "some"
// is the fraction of the interval in which at least one task was stalled
// waiting for the resource, "full" the fraction in which every task was. High
// cpu-some with moderate average utilization is the signature of a node that
// is oversubscribed in bursts.
package nodestats

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	db "sigmaos/debug"
)

const (
	// Prefix on every line, so runs can be grepped out of node logs.
	PREFIX = "NODESTATS"
)

// Start samples node state every interval and logs one line per sample, until
// the returned stop function is called. extra, if non-nil, contributes
// caller-specific fields (e.g., how many procs the caller is running), which
// is what ties node state to the packing under test.
func Start(interval time.Duration, extra func() string) (stop func()) {
	done := make(chan struct{})
	go func() {
		prev := newSample(extra)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				s := newSample(extra)
				db.DPrintf(db.ALWAYS, "%v %v", PREFIX, s.since(prev))
				prev = s
			}
		}
	}()
	return func() { close(done) }
}

type sample struct {
	t time.Time
	// PSI totals, in microseconds of stall since boot.
	cpuSome  uint64
	memSome  uint64
	memFull  uint64
	ioSome   uint64
	ioFull   uint64
	cpuBusy  uint64 // jiffies of non-idle CPU time, summed over cores
	cpuTotal uint64 // jiffies of CPU time, summed over cores
	loadavg  string
	memAvail int // MB
	extra    string
}

func newSample(extra func() string) *sample {
	s := &sample{t: time.Now()}
	s.cpuSome = psiTotal("/proc/pressure/cpu", "some")
	s.memSome = psiTotal("/proc/pressure/memory", "some")
	s.memFull = psiTotal("/proc/pressure/memory", "full")
	s.ioSome = psiTotal("/proc/pressure/io", "some")
	s.ioFull = psiTotal("/proc/pressure/io", "full")
	s.cpuBusy, s.cpuTotal = cpuJiffies()
	s.loadavg = firstFields("/proc/loadavg", 3)
	s.memAvail = meminfoMB("MemAvailable")
	if extra != nil {
		s.extra = extra()
	}
	return s
}

// since renders this sample as deltas against the previous one: the fraction
// of the interval spent stalled on each resource, and CPU utilization.
func (s *sample) since(prev *sample) string {
	d := s.t.Sub(prev.t)
	if d <= 0 {
		return "interval:0"
	}
	us := float64(d.Microseconds())
	frac := func(cur, old uint64) float64 {
		if cur < old {
			return 0
		}
		return 100.0 * float64(cur-old) / us
	}
	util := 0.0
	if s.cpuTotal > prev.cpuTotal {
		util = 100.0 * float64(s.cpuBusy-prev.cpuBusy) / float64(s.cpuTotal-prev.cpuTotal)
	}
	line := fmt.Sprintf("interval:%v cpuUtil:%.1f%% cpuSome:%.1f%% memSome:%.1f%% memFull:%.1f%% ioSome:%.1f%% ioFull:%.1f%% loadavg:%v memAvailMB:%d",
		d.Round(time.Millisecond), util,
		frac(s.cpuSome, prev.cpuSome),
		frac(s.memSome, prev.memSome), frac(s.memFull, prev.memFull),
		frac(s.ioSome, prev.ioSome), frac(s.ioFull, prev.ioFull),
		s.loadavg, s.memAvail)
	if s.extra != "" {
		line += " " + s.extra
	}
	return line
}

// psiTotal reads the "total=" counter (microseconds of stall since boot) for
// the given class ("some" or "full"). Returns 0 if PSI isn't available.
func psiTotal(pn, class string) uint64 {
	b, err := os.ReadFile(pn)
	if err != nil {
		return 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(l, class+" ") {
			continue
		}
		for _, f := range strings.Fields(l) {
			if v, ok := strings.CutPrefix(f, "total="); ok {
				n, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return 0
				}
				return n
			}
		}
	}
	return 0
}

// cpuJiffies returns (non-idle, total) jiffies from /proc/stat's summary line.
func cpuJiffies() (uint64, uint64) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(l, "cpu ") {
			continue
		}
		var total, idle uint64
		for i, f := range strings.Fields(l)[1:] {
			n, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				continue
			}
			total += n
			// Fields are user nice system idle iowait ...; idle and iowait
			// are both "not doing work".
			if i == 3 || i == 4 {
				idle += n
			}
		}
		return total - idle, total
	}
	return 0, 0
}

func firstFields(pn string, n int) string {
	b, err := os.ReadFile(pn)
	if err != nil {
		return ""
	}
	f := strings.Fields(string(b))
	if len(f) < n {
		n = len(f)
	}
	return strings.Join(f[:n], ",")
}

func meminfoMB(pat string) int {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(l, pat+":") {
			continue
		}
		f := strings.Fields(l)
		if len(f) < 2 {
			return 0
		}
		kb, err := strconv.Atoi(f[1])
		if err != nil {
			return 0
		}
		return kb / 1024
	}
	return 0
}
