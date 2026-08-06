package kernel

// Periodic CPU accounting for everything running on a node, so that a workload
// suspected of spending its cycles on infrastructure rather than on its own work
// can be attributed. The case it was written for is a very fine-grained MR job,
// whose procs are short enough that starting one plausibly costs more than
// running it: this says how much of the node goes to msched, to besched, to the
// realm's fsuxd/fss3d/chunkd, to procd and binsrv, and how much to the procs
// themselves.
//
// Attribution is per *process*, because that is what σOS's services are: only
// procd runs in a container of its own (proc.HDOCKER), while msched, besched,
// lcsched, named and spproxyd are plain processes in the kernel's container
// (proc.HLINUX) and fsuxd, fss3d and chunkd are procs run through msched
// (proc.HMSCHED), so they share procd's cgroup with the user procs. Reading
// cgroups alone would therefore report one number for the whole node and one for
// each procd — which is why the per-cgroup figures here are a supplement, giving
// the per-realm rollup that command names can't.
//
// The kernel is where this lives rather than in each daemon: it is started with
// --pid host (see start-kernel.sh), so it sees every process on its node, which
// no individual daemon does. One monitor thread therefore covers every service,
// including ones added later and ones that aren't σOS's at all (dockerd, etcd),
// and its total can be checked against what /proc/stat says the node is doing.
//
// Off unless CPU_MON is selected.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"sigmaos/dcontainer/cgroup"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	linuxsched "sigmaos/util/linux/sched"
	"sigmaos/util/perf"
)

const (
	// 10Hz, because the procs this exists to account for are short: a proc that
	// starts and exits between two rounds is invisible, and at 1Hz that was most of
	// a fine-grained job's mappers. A proc caught by even one round contributes all
	// the CPU it used up to that read, so the shortfall per proc is at most one
	// round's worth of its tail.
	//
	// The cost is a /proc/<pid>/stat read per process per round — a few thousand
	// small reads a second on a busy node — which is why this is behind a debug
	// selector rather than on by default.
	cpuMonInterval = 100 * time.Millisecond
	// How many commands the per-command line names before summing the rest into
	// "other": enough to cover σOS's services and a job's procs, not so many that
	// the line is unreadable on a busy node.
	cpuMonTopComms = 16
	// Below this, a cgroup doesn't get a line of its own: rounded to one decimal it
	// would read as 0.0 anyway.
	cpuMonMinCores = 0.05
	// The unit /proc/<pid>/stat reports CPU time in. 100Hz is the near-universal
	// Linux configuration, and asking for certain (sysconf(_SC_CLK_TCK)) needs
	// cgo, so a kernel configured otherwise makes the per-process figures wrong by
	// that ratio. The node total and the cgroup figures don't depend on it, so a
	// wrong USER_HZ shows up as a per-command total that doesn't add up to the
	// node — worth a glance before trusting a surprising breakdown.
	//
	// It also sets the resolution: one jiffy is 0.1 core over a 100ms round, so a
	// single process's figure on a single line quantizes to steps of 0.1. Jiffies
	// accumulate rather than being lost, so sums over a phase are unaffected — a
	// process using a tenth of a core reads 0.0 for nine rounds and 0.1 on the
	// tenth. Read the per-line breakdown as a distribution, not as an instant.
	jiffiesPerSec = 100.0
)

// startCPUMon starts the CPU monitor, if it was asked for.
func (k *Kernel) startCPUMon() {
	if !db.WillBePrinted(db.CPU_MON) {
		return
	}
	db.DPrintf(db.CPU_MON, "start CPU monitor: kernel %v interval %v ncores %v", k.Param.KernelID, cpuMonInterval, linuxsched.GetNCores())
	go k.cpuMon()
}

func (k *Kernel) cpuMon() {
	// Checked again here, not only in startCPUMon: everything this monitor does —
	// the cgroup reads, the /proc scan of every process on the node ten times a
	// second — is unconditional once this function is entered, so the selector
	// that turns it on belongs to the function that does the work rather than to
	// whoever calls it.
	if !db.WillBePrinted(db.CPU_MON) {
		return
	}
	// Delta state of our own, deliberately not the CgroupMonitor each DContainer
	// keeps: GetCPUStats reports the CPU used since its own last call for a
	// cgroup, so sharing state with the realm-utilization path
	// (ProcdMgr.GetCPUUtil, which the benchmarks poll every second) would leave
	// each of the two measuring only the interval since the other's last poll.
	cmon := cgroup.NewCgroupMonitor()
	cores := perf.GetActiveCores()
	ncores := float64(linuxsched.GetNCores())
	prevIdle, prevTotal := perf.GetCPUSample(cores)
	procs := newProcCPUSampler()
	// The first round has nothing to take deltas against, so every process counts
	// the CPU it used since it started — which for one that has been up for hours
	// is a nonsense figure (and makes the unsampled remainder hugely negative). Do
	// the round, to have something to compare the second against, but don't report
	// it.
	priming := true
	last := time.Now()

	t := time.NewTicker(cpuMonInterval)
	defer t.Stop()
	for range t.C {
		if k.isShuttingDown() {
			db.DPrintf(db.CPU_MON, "CPU monitor exiting: kernel %v shutting down", k.Param.KernelID)
			return
		}
		now := time.Now()
		dt := now.Sub(last)
		last = now

		// What the whole node is doing, to measure the breakdown against.
		idle, total := perf.GetCPUSample(cores)
		nodeCores := 0.0
		if dTotal := total - prevTotal; dTotal > 0 {
			nodeCores = (1.0 - float64(idle-prevIdle)/float64(dTotal)) * ncores
		}
		prevIdle, prevTotal = idle, total

		// Every process on the node, by command name.
		sm := procs.sample(allPids(), dt)
		if !priming {
			g := groupCores(sm.byComm)
			db.DPrintf(db.CPU_MON, "node %.1f/%.0f cores over %.0fms; sampled %.1f over %v procs (%v new); unsampled %.1f; procd %.1f; sigma %.1f %v; platform %.1f %v; procs %.1f %v",
				nodeCores, ncores, float64(dt.Microseconds())/1000.0, sm.cores, sm.npids, sm.nnew, nodeCores-sm.cores,
				g.procd,
				sum(g.sigma), fmtCores(g.sigma, nil, 0),
				sum(g.platform), fmtCores(g.platform, nil, 0),
				sum(g.rest), fmtCores(g.rest, nil, cpuMonTopComms))
		}

		// The containerized subsystems, which is procd. A procd cgroup's figure is
		// essentially the aggregate utilization of the procs running under it, since
		// they share its cgroup — procd, spproxyd and wasmd themselves are named
		// separately in the breakdown, and are small. So node cores minus the procd
		// cgroups is what the infrastructure outside the sandboxes costs, which is
		// what limits how much of a node the procs can have. One line per container
		// so that a realm's procs can be told from another's.
		for _, sub := range k.cgroupSubsystems() {
			st, err := cmon.GetCPUStats(sub.path)
			if err != nil {
				db.DPrintf(db.CPU_MON, "err CPU stats %v: %v", sub.label, err)
				continue
			}
			// Utilization comes back as a percentage of one core (100 = one core
			// busy); report cores, as the benchmarks do.
			c := st.Util / 100.0
			// A node runs a procd per realm and most of them are idle at any moment
			// (and their delta state still has to be kept up to date, which
			// GetCPUStats just did), so don't spend a line on one that did nothing.
			if c < cpuMonMinCores {
				continue
			}
			pids, err := cmon.GetPIDs(sub.path)
			if err != nil {
				db.DPrintf(db.CPU_MON, "  %v %.1f cores (err cgroup pids: %v)", sub.label, c, err)
				continue
			}
			csm := sm.subset(pids)
			g := groupCores(csm.byComm)
			// c is the whole cgroup, so c - procd - sigma - platform - procs is the
			// part of it no sample caught, which is the procs too short-lived to see.
			db.DPrintf(db.CPU_MON, "  %v %.1f cores over %v procs (%v new); unsampled %.1f; procd %.1f; sigma %.1f %v; platform %.1f %v; procs %.1f %v",
				sub.label, c, csm.npids, csm.nnew, c-csm.cores,
				g.procd,
				sum(g.sigma), fmtCores(g.sigma, nil, 0),
				sum(g.platform), fmtCores(g.platform, nil, 0),
				sum(g.rest), fmtCores(g.rest, nil, cpuMonTopComms))
		}
		procs.endTick()
		priming = false
	}
}

func (k *Kernel) isShuttingDown() bool {
	k.Lock()
	defer k.Unlock()
	return k.shuttingDown
}

// cgroupSub is one measurable subsystem: a label to report it under and the
// cgroup to read.
type cgroupSub struct {
	label string
	path  string
}

// cgroupSubsystems is the kernel's subsystems that have a cgroup of their own,
// labelled by service, realm (when it isn't the root realm's) and pid, since a
// node runs a procd per realm and they are all booted as root-realm subsystems.
func (k *Kernel) cgroupSubsystems() []cgroupSub {
	k.Lock()
	defer k.Unlock()

	if k.shuttingDown {
		return nil
	}
	subs := make([]cgroupSub, 0)
	for s, ssrv := range k.svcs.svcs {
		for _, ss := range ssrv {
			// Not every Subsystem implementation is measurable, and one the kernel
			// started outside a container has no cgroup.
			cg, ok := ss.(interface{ cgroupPath() (string, bool) })
			if !ok {
				continue
			}
			path, ok := cg.cgroupPath()
			if !ok {
				continue
			}
			p := ss.GetProc()
			label := s
			if r := p.GetRealm(); r != sp.ROOTREALM {
				label = s + "/" + r.String()
			}
			subs = append(subs, cgroupSub{label: label + ":" + p.GetPid().String(), path: path})
		}
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].label < subs[j].label })
	return subs
}

// sigmaDaemons are σOS's own servers, by the command name they run under — the
// programs in bin/kernel, plus the per-proc servers procd starts. Named
// explicitly, and reported as a group of their own that is always listed in full,
// so that the cost of a particular server can be read off every line: a top-N
// list would drop whichever of them was quiet this round, which is exactly the
// comparison ("fsuxd vs fss3d vs chunkd") the breakdown exists to support.
//
// Not everything with a name is a process: binsrv, whose filesystem shows up as
// "binfs", is a FUSE server running inside procd (sched/msched/proc/srv/binsrv),
// so its CPU is part of procd's figure and no amount of sampling separates it.
// Same for anything else that is a goroutine rather than a proc — Go doesn't give
// threads their own comm, so even per-thread accounting wouldn't split it.
// Separating those needs instrumentation inside the server.
var sigmaDaemons = map[string]bool{
	"besched":  true,
	"chunkd":   true,
	"dbd":      true,
	"fss3d":    true,
	"fsuxd":    true,
	"knamed":   true,
	"lcsched":  true,
	"mongod":   true,
	"msched":   true,
	"named":    true,
	"realmd":   true,
	"spproxyd": true,
	"wasmd":    true,
	// The kernel process itself, which also runs this monitor: at 10Hz the
	// sampling is a few percent of a core, and it lands here.
	"bootkernel": true,
}

// procdComm gets a group to itself: procd is the one σOS server whose cost has to
// be read apart both from the other servers and from what runs under it, since the
// procs it starts share its cgroup and this is the only figure that separates the
// two. It is procd's own processes only — one per realm on a node, summed — and it
// includes binfs, per the note above.
const procdComm = "procd"

// platformDaemons are the things underneath σOS that a proc's lifecycle drives
// but that σOS doesn't implement: the container runtime and the store named runs
// on. Grouped separately because they are infrastructure cost, but not σOS's, and
// like sigmaDaemons they are always listed in full.
var platformDaemons = map[string]bool{
	"containerd":      true,
	"containerd-shim": true,
	"docker":          true,
	"dockerd":         true,
	"etcd":            true,
}

// coreGroups is a round's CPU split into the things worth telling apart: procd
// itself, σOS's other servers, the platform's daemons, and everything else — which
// on a worker node is the procs running under procd, plus the node's own processes
// and kernel threads.
type coreGroups struct {
	procd    float64
	sigma    map[string]float64
	platform map[string]float64
	rest     map[string]float64
}

func groupCores(byComm map[string]float64) coreGroups {
	g := coreGroups{
		sigma:    make(map[string]float64),
		platform: make(map[string]float64),
		rest:     make(map[string]float64),
	}
	for c, v := range byComm {
		switch {
		case c == procdComm:
			g.procd += v
		case sigmaDaemons[c]:
			g.sigma[c] = v
		case platformDaemons[c]:
			g.platform[c] = v
		default:
			g.rest[c] = v
		}
	}
	return g
}

func sum(m map[string]float64) float64 {
	t := 0.0
	for _, v := range m {
		t += v
	}
	return t
}

// fmtCores renders a label -> cores map biggest-first, so that whatever is
// costing the most is at the front of the line. topN > 0 sums everything past the
// biggest topN into "other", to keep the line readable on a busy node; labels
// that used no CPU this round are left out entirely. nsubs, when given, says how
// many things a label covers.
func fmtCores(byLabel map[string]float64, nsubs map[string]int, topN int) string {
	labels := make([]string, 0, len(byLabel))
	for l := range byLabel {
		labels = append(labels, l)
	}
	sort.Slice(labels, func(i, j int) bool {
		if byLabel[labels[i]] != byLabel[labels[j]] {
			return byLabel[labels[i]] > byLabel[labels[j]]
		}
		return labels[i] < labels[j]
	})
	parts := make([]string, 0, len(labels))
	other, nother := 0.0, 0
	for i, l := range labels {
		c := byLabel[l]
		if c == 0 {
			continue
		}
		if topN > 0 && i >= topN {
			other += c
			nother++
			continue
		}
		if n := nsubs[l]; n > 1 {
			parts = append(parts, fmt.Sprintf("%s(x%d) %.1f", l, n, c))
		} else {
			parts = append(parts, fmt.Sprintf("%s %.1f", l, c))
		}
	}
	if nother > 0 {
		parts = append(parts, fmt.Sprintf("other(x%d) %.1f", nother, other))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// allPids is every process on the node, which the kernel can see because it runs
// in the host's pid namespace.
func allPids() []int {
	f, err := os.Open("/proc")
	if err != nil {
		db.DPrintf(db.CPU_MON, "err open /proc: %v", err)
		return nil
	}
	defer f.Close()
	names, err := f.Readdirnames(0)
	if err != nil {
		db.DPrintf(db.CPU_MON, "err read /proc: %v", err)
		return nil
	}
	pids := make([]int, 0, len(names))
	for _, n := range names {
		// /proc holds a numeric directory per process and much else besides.
		if pid, err := strconv.Atoi(n); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// procKey identifies a sampled process. The start time is part of it because a
// pid the kernel has recycled would otherwise look like the process that had it
// before, and its CPU time would appear to jump backwards.
type procKey struct {
	pid       int
	starttime uint64
}

// procSample is one round's per-process reading.
type procSample struct {
	byComm map[string]float64 // cores used since the previous round, by command
	byPid  map[int]pidCPU     // the same, per process, for subsetting by cgroup
	cores  float64            // their total
	npids  int                // processes read
	nnew   int                // of those, ones not seen in the previous round
}

type pidCPU struct {
	comm  string
	cores float64
	isNew bool
}

// subset is the part of this sample belonging to the given processes — a cgroup's,
// say. Taken from the round already read rather than by reading those processes
// again: at 10Hz a stat read per process per round is the monitor's whole cost,
// and reading procd's pids a second time would nearly double it on a busy node.
// Processes that have exited since the round are simply absent.
func (sm procSample) subset(pids []int) procSample {
	sub := procSample{byComm: make(map[string]float64)}
	for _, pid := range pids {
		p, ok := sm.byPid[pid]
		if !ok {
			continue
		}
		sub.npids++
		if p.isNew {
			sub.nnew++
		}
		sub.byComm[p.comm] += p.cores
		sub.cores += p.cores
	}
	return sub
}

// procCPUSampler attributes CPU to command names by sampling /proc/<pid>/stat.
//
// This is a sample, and deliberately reported as one: a process that starts and
// exits between two rounds is never seen, so its CPU appears only in the node
// total (or its cgroup's). For a fine-grained job that is not a rounding error —
// the procs are the short-lived things — which is why the shortfall is logged as
// "unsampled" rather than quietly dropped. Read with the count of new pids per
// round, it is what says whether a node's cycles are going to the procs or to the
// long-lived machinery that starts them.
type procCPUSampler struct {
	prev map[procKey]uint64 // total CPU jiffies, as of the previous round
	cur  map[procKey]uint64
	// How a process's stat is read, so that a test can drive the delta
	// bookkeeping without processes to read.
	read func(pid int) (comm string, cpu uint64, starttime uint64, ok bool)
}

func newProcCPUSampler() *procCPUSampler {
	return &procCPUSampler{
		prev: make(map[procKey]uint64),
		cur:  make(map[procKey]uint64),
		read: readProcStat,
	}
}

// sample reads the given processes and returns the CPU they have used since the
// previous round, in cores over the dt that round took, summed by command name. A
// process first seen now counts the CPU it has used since it started, which for
// one that started within the round is right and for one that was already running
// (the first round, or a process whose read failed last round) overstates it.
//
// dt is measured rather than assumed to be cpuMonInterval: at 10Hz a round that
// reads a few thousand stat files can overrun its tick, and dividing by the
// nominal interval would inflate everything by the overrun.
func (s *procCPUSampler) sample(pids []int, dt time.Duration) procSample {
	sm := procSample{
		byComm: make(map[string]float64),
		byPid:  make(map[int]pidCPU, len(pids)),
	}
	for _, pid := range pids {
		comm, cpu, starttime, ok := s.read(pid)
		if !ok {
			// Exited while we were reading it; its CPU is in the node total.
			continue
		}
		sm.npids++
		k := procKey{pid: pid, starttime: starttime}
		delta := cpu
		isNew := false
		if prev, ok := s.prev[k]; ok {
			if cpu < prev {
				// Can't happen for a live process, but a rewound counter would
				// otherwise underflow into a nonsense figure.
				delta = 0
			} else {
				delta = cpu - prev
			}
		} else {
			isNew = true
			sm.nnew++
		}
		s.cur[k] = cpu
		c := float64(delta) / jiffiesPerSec / dt.Seconds()
		sm.byComm[comm] += c
		sm.byPid[pid] = pidCPU{comm: comm, cores: c, isNew: isNew}
		sm.cores += c
	}
	return sm
}

// endTick makes this round's readings the ones the next round takes deltas
// against. Called once a round, after every sample it took — not per sample, or
// each would discard the readings of the ones before it and see their processes
// as new. Processes that have gone are dropped here, so that the state doesn't
// grow with every proc the node has ever run.
func (s *procCPUSampler) endTick() {
	s.prev, s.cur = s.cur, s.prev
	for k := range s.cur {
		delete(s.cur, k)
	}
}

// readProcStat reads a process's command name, total CPU time (in jiffies) and
// start time. Not ok if the process has gone, or its stat is unparseable.
func readProcStat(pid int) (comm string, cpu uint64, starttime uint64, ok bool) {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", 0, 0, false
	}
	return parseProcStat(string(b))
}

// parseProcStat pulls the command name, total CPU time (utime + stime, in
// jiffies) and start time out of a /proc/<pid>/stat line. The command sits in
// parentheses and may itself contain spaces and parentheses, so the fields are
// counted from the last ')' rather than by splitting the whole line.
func parseProcStat(b string) (comm string, cpu uint64, starttime uint64, ok bool) {
	i := strings.IndexByte(b, '(')
	j := strings.LastIndexByte(b, ')')
	if i < 0 || j < i+1 {
		return "", 0, 0, false
	}
	comm = b[i+1 : j]
	// The fields after the command are stat's third onwards, so f[0] is "state",
	// f[11] utime (field 14), f[12] stime (15) and f[19] starttime (22).
	f := strings.Fields(b[j+1:])
	if len(f) < 20 {
		return "", 0, 0, false
	}
	utime, err := strconv.ParseUint(f[11], 10, 64)
	if err != nil {
		return "", 0, 0, false
	}
	stime, err := strconv.ParseUint(f[12], 10, 64)
	if err != nil {
		return "", 0, 0, false
	}
	st, err := strconv.ParseUint(f[19], 10, 64)
	if err != nil {
		return "", 0, 0, false
	}
	return comm, utime + stime, st, true
}
