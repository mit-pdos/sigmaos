package kernel

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	linuxsched "sigmaos/util/linux/sched"
)

func TestParseProcStatSelf(t *testing.T) {
	b, err := os.ReadFile("/proc/self/stat")
	assert.Nil(t, err)
	comm, cpu, starttime, ok := parseProcStat(string(b))
	assert.True(t, ok)
	// The test binary's name is truncated to 15 characters in stat, so check a
	// prefix rather than the whole thing.
	assert.Contains(t, comm, "kernel")
	assert.Greater(t, starttime, uint64(0))
	// A process that has done anything at all has used some CPU, but it may be
	// less than a jiffy, so only the read is checked.
	_ = cpu
}

// A command name can contain spaces and parentheses, which is why the fields are
// counted from the last ')' rather than by splitting the line.
func TestParseProcStatAwkwardComm(t *testing.T) {
	// pid, comm, then state and the rest: utime is the 12th field after the comm,
	// stime the 13th, starttime the 20th.
	fields := "S 1 2 3 4 -1 0 0 0 0 0 17 25 0 0 20 0 1 0 991177"
	comm, cpu, starttime, ok := parseProcStat("42 ((weird) proc name) " + fields + " 0 0 0\n")
	assert.True(t, ok)
	assert.Equal(t, "(weird) proc name", comm)
	assert.Equal(t, uint64(17+25), cpu)
	assert.Equal(t, uint64(991177), starttime)
}

func TestParseProcStatMalformed(t *testing.T) {
	for _, s := range []string{"", "42", "42 (proc) S 1 2 3", "42 proc S 1 2 3 4", ")("} {
		_, _, _, ok := parseProcStat(s)
		assert.False(t, ok, "should not parse %q", s)
	}
}

// The interesting part of the sampler is its bookkeeping: CPU is reported per
// interval, a process first seen counts from its start, one that has gone stops
// being counted, and a recycled pid isn't mistaken for its predecessor.
func TestProcCPUSampler(t *testing.T) {
	type fake struct {
		comm      string
		cpu       uint64
		starttime uint64
		gone      bool
	}
	procs := map[int]*fake{}
	s := newProcCPUSampler()
	s.read = func(pid int) (string, uint64, uint64, bool) {
		f, ok := procs[pid]
		if !ok || f.gone {
			return "", 0, 0, false
		}
		return f.comm, f.cpu, f.starttime, true
	}
	// A round of one second, so that a process using a full core accumulates
	// jiffiesPerSec jiffies over it. Written out rather than taken from
	// cpuMonInterval so that retuning the sampling rate doesn't change what this
	// test is asserting.
	const round = time.Second
	full := uint64(jiffiesPerSec * round.Seconds())

	procs[1] = &fake{comm: "procd", cpu: full, starttime: 10}
	procs[2] = &fake{comm: "mapper", cpu: 2 * full, starttime: 11}

	// First round: both are new, so both count everything they have used.
	sm := s.sample([]int{1, 2}, round)
	assert.Equal(t, 1.0, sm.byComm["procd"])
	assert.Equal(t, 2.0, sm.byComm["mapper"])
	assert.Equal(t, 3.0, sm.cores)
	assert.Equal(t, 2, sm.npids)
	assert.Equal(t, 2, sm.nnew)
	s.endTick()

	// Second round: only what they used since. procd was idle; a second mapper
	// appeared and used half a core.
	procs[2].cpu += full
	procs[3] = &fake{comm: "mapper", cpu: full / 2, starttime: 12}
	sm = s.sample([]int{1, 2, 3}, round)
	assert.Equal(t, 0.0, sm.byComm["procd"])
	assert.Equal(t, 1.5, sm.byComm["mapper"], "one mapper's full core plus a new one's half")
	assert.Equal(t, 1.5, sm.cores)
	assert.Equal(t, 1, sm.nnew, "only the mapper that just appeared")
	s.endTick()

	// Third round: the mappers have gone and pid 2 has been recycled by a new
	// process, whose lower CPU total must not read as a negative delta.
	procs[3].gone = true
	procs[2] = &fake{comm: "sleeper", cpu: full / 4, starttime: 99}
	sm = s.sample([]int{1, 2, 3}, round)
	assert.Equal(t, 0.25, sm.byComm["sleeper"])
	assert.NotContains(t, sm.byComm, "mapper")
	assert.Equal(t, 0.25, sm.cores)
	assert.Equal(t, 2, sm.npids, "the exited mapper isn't readable")
	s.endTick()

	// The state holds only what is still running: pid 1 and the recycled pid 2.
	assert.Len(t, s.prev, 2)
}

// A cgroup's share of a round is taken from the round already read, rather than by
// reading those processes again.
func TestProcSampleSubset(t *testing.T) {
	sm := procSample{
		byComm: map[string]float64{"procd": 0.1, "mr-m": 2.0},
		byPid: map[int]pidCPU{
			1: {comm: "procd", cores: 0.1},
			2: {comm: "mr-m", cores: 1.5, isNew: true},
			3: {comm: "mr-m", cores: 0.5, isNew: true},
		},
		cores: 2.1,
		npids: 3,
		nnew:  2,
	}
	// A pid in the cgroup that the round didn't see (it exited) contributes nothing
	// rather than being counted as zero-CPU.
	sub := sm.subset([]int{2, 3, 99})
	assert.Equal(t, 2.0, sub.cores)
	assert.Equal(t, 2.0, sub.byComm["mr-m"])
	assert.NotContains(t, sub.byComm, "procd")
	assert.Equal(t, 2, sub.npids)
	assert.Equal(t, 2, sub.nnew)
}

// A sample can't see a process that started and exited between two rounds, so it
// must not claim it accounted for the cgroup: the shortfall is what the caller
// reports as unsampled.
func TestProcCPUSamplerMissesShortLivedProcs(t *testing.T) {
	s := newProcCPUSampler()
	s.read = func(pid int) (string, uint64, uint64, bool) {
		return "", 0, 0, false
	}
	sm := s.sample([]int{1, 2, 3}, time.Second)
	assert.Len(t, sm.byComm, 0)
	assert.Equal(t, 0.0, sm.cores)
	assert.Equal(t, 0, sm.npids)
}

// The sampler against the real /proc: a process burning a core for a round should
// be attributed roughly a core, which is what says the jiffy arithmetic and the
// stat parsing agree with each other on live processes.
func TestProcCPUSamplerAgainstProc(t *testing.T) {
	s := newProcCPUSampler()
	pids := allPids()
	assert.Greater(t, len(pids), 1, "should see the node's processes")
	s.sample(pids, time.Second)
	s.endTick()

	start := time.Now()
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
		}
	}()
	// Long enough that the jiffy resolution (a hundredth of a core-second) doesn't
	// dominate; the sampling rate itself is finer than this.
	time.Sleep(500 * time.Millisecond)
	close(done)

	sm := s.sample(allPids(), time.Since(start))
	self, err := os.ReadFile("/proc/self/stat")
	assert.Nil(t, err)
	comm, _, _, ok := parseProcStat(string(self))
	assert.True(t, ok)
	// One busy goroutine, plus whatever the test itself is doing: at least most of
	// a core, and not more than the machine has.
	assert.Greater(t, sm.byComm[comm], 0.5, "this process should show as busy")
	assert.LessOrEqual(t, sm.cores, float64(linuxsched.GetNCores()))
}

// What a round costs, which is what decides whether the sampling rate is
// affordable: at 10Hz the monitor spends this much of a core, times 10, on every
// node it runs on.
func BenchmarkProcCPUSample(b *testing.B) {
	s := newProcCPUSampler()
	pids := allPids()
	b.ReportMetric(float64(len(pids)), "procs/round")
	for i := 0; i < b.N; i++ {
		s.sample(pids, cpuMonInterval)
		s.endTick()
	}
}

// procd is read apart from the other servers and from what runs under it, and
// every σOS server is named whatever it cost, so that a quiet one still shows up
// in the line rather than being dropped into "other".
func TestGroupCores(t *testing.T) {
	g := groupCores(map[string]float64{
		"procd":           0.4,
		"fsuxd":           1.2,
		"fss3d":           0.3,
		"chunkd":          0.0,
		"spproxyd":        0.2,
		"msched":          0.5,
		"dockerd":         0.7,
		"containerd-shim": 0.1,
		"mr-m-grep-v1.0":  8.0,
		"kswapd0":         0.1,
	})
	assert.Equal(t, 0.4, g.procd, "procd on its own, not with the other servers")
	assert.NotContains(t, g.sigma, "procd")
	assert.InDelta(t, 2.2, sum(g.sigma), 1e-9, "fsuxd+fss3d+chunkd+spproxyd+msched")
	assert.InDelta(t, 0.8, sum(g.platform), 1e-9, "dockerd+containerd-shim")
	assert.InDelta(t, 8.1, sum(g.rest), 1e-9, "the procs, and the node's own threads")
	// A server that used nothing is still in the group (fmtCores drops the zero
	// from the line, but the group total stays comparable across rounds).
	assert.Contains(t, g.sigma, "chunkd")
	assert.Contains(t, g.rest, "mr-m-grep-v1.0")
}

// sigmaDaemons is a hand-written list, so if it and the kernel's programs drift
// apart the breakdown silently starts filing a server under "procs" — which is how
// msched went missing from it once already. Check the names the kernel boots.
func TestSigmaDaemonsCoversKernelPrograms(t *testing.T) {
	// The programs bootSubsystem is called with, plus procd's own helpers.
	for _, program := range []string{
		"besched", "lcsched", "msched", "realmd", "fsuxd", "fss3d", "chunkd",
		"dbd", "mongod", "knamed", "named", "spproxyd", "wasmd",
	} {
		assert.True(t, sigmaDaemons[program] || program == procdComm,
			"%v is a σOS server but wouldn't be grouped as one", program)
	}
}

func TestFmtCores(t *testing.T) {
	byLabel := map[string]float64{"fsuxd": 2.5, "msched": 10.0, "fss3d": 0.5, "idle": 0.0}
	// Biggest first, and a command that used nothing is left out.
	assert.Equal(t, "[msched 10.0 fsuxd 2.5 fss3d 0.5]", fmtCores(byLabel, nil, 0))
	assert.Equal(t, "[msched(x3) 10.0 fsuxd 2.5 fss3d 0.5]", fmtCores(byLabel, map[string]int{"msched": 3}, 0))
	// Past topN, the rest is summed rather than named.
	assert.Equal(t, "[msched 10.0 other(x2) 3.0]", fmtCores(byLabel, nil, 1))
}
