// Package memblock takes machines out of the pool of nodes that will accept
// procs, by occupying their scheduler's memory budget with a proc of its own.
//
// Written for the MR experiment that dedicates machines to hosting the job's
// input in their fsuxd servers (claude-slop/DEDICATED_UX_MACHINES.md): those
// machines must serve reads and writes but run no mappers or reducers. Nothing
// here is MR-specific — it is "make these nodes refuse work" — so it is also what
// to reach for to isolate any server from the procs that would otherwise share
// its machine.
//
// # Budget, not bytes
//
// A node stops accepting procs when msched's memory budget (what besched filters
// candidates on) drops below what a proc requests. That budget is accounting, not
// occupancy: `Proc.SetMem` reserves it, while the `memblock` program's argument is
// how much memory it actually allocates and touches.
//
// So a blocker requests the whole budget and, by default, allocates almost none
// of it. Physically allocating a machine's memory would be actively harmful here:
// it evicts the page cache the dedicated fsuxd is serving from, and it risks the
// OOM killer taking the very server the machine exists to run. A caller that
// genuinely wants the memory gone (to model a memory-starved node, say) can ask
// for it with WithAllocMem.
package memblock

import (
	"fmt"
	"sort"

	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

const (
	// PROGRAM is the proc that does the occupying (cmd/user/memblock).
	PROGRAM = "memblock"
	// How much memory a blocker actually allocates and touches, as opposed to how
	// much it reserves from the scheduler. Small on purpose: see the package
	// comment. Not zero, because the point of a resident allocation is to be
	// resident, and it makes the proc's own accounting non-degenerate.
	DEFAULT_ALLOC_MEM proc.Tmem = 1
	// How many blockers to try per node before giving up. More than one is needed
	// because a proc can be admitted between the query and the blocker landing, so
	// the first blocker can leave a remainder; many more than one means something
	// else is actively filling the node and the caller should hear about it rather
	// than have us spin.
	DEFAULT_MAX_ROUNDS = 4
)

// Blocker blocks nodes and remembers what it spawned, so that the blocks can be
// undone together.
type Blocker struct {
	sc        *sigmaclnt.SigmaClnt
	msc       *mschedclnt.MSchedClnt
	allocMem  proc.Tmem
	maxRounds int
	pids      []sp.Tpid
}

type Opt func(*Blocker)

// WithAllocMem sets how much memory each blocker really allocates and touches.
// Only for a caller that wants the memory physically consumed; blocking a node
// from accepting procs does not need it. See the package comment.
func WithAllocMem(m proc.Tmem) Opt {
	return func(b *Blocker) { b.allocMem = m }
}

// WithMaxRounds sets how many blockers to try per node before reporting failure.
func WithMaxRounds(n int) Opt {
	return func(b *Blocker) { b.maxRounds = n }
}

func NewBlocker(sc *sigmaclnt.SigmaClnt, opts ...Opt) *Blocker {
	b := &Blocker{
		sc:        sc,
		msc:       mschedclnt.NewMSchedClnt(sc.FsLib, sp.NOT_SET),
		allocMem:  DEFAULT_ALLOC_MEM,
		maxRounds: DEFAULT_MAX_ROUNDS,
		pids:      make([]sp.Tpid, 0),
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// Kernels returns the first n kernel IDs in the cluster, sorted, or all of them
// if n <= 0. Sorted so that a repeated run of the same experiment blocks the same
// machines: which nodes are dedicated is then a property of the cluster, not of
// the order msched's directory happened to be read in.
func (b *Blocker) Kernels(n int) ([]string, error) {
	kids, err := b.msc.GetMScheds()
	if err != nil {
		return nil, err
	}
	return firstN(kids, n)
}

// firstN sorts kids and takes the first n of them, or all of them if n <= 0.
// Asking for more than the cluster has is an error rather than "take what there
// is": a caller sizing an experiment around n dedicated machines gets a wrong
// experiment, not a smaller one, if it silently receives fewer.
func firstN(kids []string, n int) ([]string, error) {
	sort.Strings(kids)
	if n <= 0 {
		return kids, nil
	}
	if n > len(kids) {
		return nil, fmt.Errorf("memblock: asked for %d kernels, cluster has %d (%v)", n, len(kids), kids)
	}
	return kids[:n], nil
}

// BlockKernels makes each of kids refuse procs that request reject or less
// memory, and returns once they all do. On failure the blockers spawned so far
// are left in place and recorded, so Evict still cleans up.
func (b *Blocker) BlockKernels(kids []string, reject proc.Tmem) error {
	db.DPrintf(db.ALWAYS, "memblock: blocking %v kernels %v against procs requesting <= %vMB", len(kids), kids, reject)
	for _, kid := range kids {
		if err := b.BlockKernel(kid, reject); err != nil {
			return err
		}
	}
	return nil
}

// BlockKernel occupies kid's memory budget until its msched would not admit a
// proc requesting reject MB, and returns an error if it can't get there.
//
// The loop is the verification, not just the mechanism: the caller's experiment
// is only valid once the node actually refuses the procs it is meant to refuse,
// and the only way to know that is to ask msched again after blocking.
func (b *Blocker) BlockKernel(kid string, reject proc.Tmem) error {
	for round := 0; round < b.maxRounds; round++ {
		free, total, err := b.msc.GetMem(kid)
		if err != nil {
			return fmt.Errorf("memblock: GetMem %v: %v", kid, err)
		}
		if free < reject {
			db.DPrintf(db.ALWAYS, "memblock: %v blocked after %v blockers: memFree %vMB < %vMB (memTotal %vMB)", kid, round, free, reject, total)
			return nil
		}
		db.DPrintf(db.ALWAYS, "memblock: %v round %v: memFree %vMB of %vMB, blocking it", kid, round, free, total)
		if err := b.spawn(kid, free); err != nil {
			return err
		}
	}
	free, _, err := b.msc.GetMem(kid)
	return fmt.Errorf("memblock: %v still admits procs requesting %vMB after %v blockers (memFree %vMB, err %v); something else is freeing memory on it",
		kid, reject, b.maxRounds, free, err)
}

// BlockAmount occupies exactly mem MB on each of kids, rather than however much
// it takes to make the node refuse work.
//
// This is the other reason to block memory: not to stop procs from being placed,
// but to run an experiment on nodes with less memory than they really have. With
// WithAllocMem set to the same figure it also takes the memory out of the machine
// for real, page cache included, which is what "the node has less memory" has to
// mean for it to be a fair model.
func (b *Blocker) BlockAmount(kids []string, mem proc.Tmem) error {
	db.DPrintf(db.ALWAYS, "memblock: occupying %vMB on %v kernels %v (allocating %vMB of it)", mem, len(kids), kids, b.allocMem)
	for _, kid := range kids {
		if err := b.spawn(kid, mem); err != nil {
			return err
		}
	}
	return nil
}

// spawn starts one blocker pinned to kid, reserving mem, and waits for it to be
// running. memblock registers itself at name/memblock/<kernelID> before it
// allocates and calls Started, so a returned WaitStart means both that the
// reservation is in effect and that the registration is visible to anyone reading
// that directory.
func (b *Blocker) spawn(kid string, mem proc.Tmem) error {
	p := proc.NewProc(PROGRAM, []string{fmt.Sprintf("%dMB", b.allocMem)})
	// besched only offers a proc to an msched the proc names: see
	// Proc.HasKernelPref, sched/besched/srv/srv.go.
	p.SetKernels([]string{kid})
	p.SetMem(mem)
	if err := b.sc.Spawn(p); err != nil {
		return fmt.Errorf("memblock: spawn on %v: %v", kid, err)
	}
	// Recorded before the wait: a blocker that was spawned but didn't come up
	// still has to be cleaned up.
	b.pids = append(b.pids, p.GetPid())
	if err := b.sc.WaitStart(p.GetPid()); err != nil {
		return fmt.Errorf("memblock: waitstart on %v: %v", kid, err)
	}
	return nil
}

// Pids are the blockers this Blocker spawned.
func (b *Blocker) Pids() []sp.Tpid {
	return b.pids
}

// Evict releases every block this Blocker holds, and waits for the blockers to
// exit so that the memory is back in the budget by the time it returns. Errors
// from individual blockers are logged and the rest are still evicted: leaving a
// blocker running would take a machine out of the cluster for every later test.
func (b *Blocker) Evict() error {
	var err1 error
	for _, pid := range b.pids {
		if err := b.sc.Evict(pid); err != nil {
			db.DPrintf(db.ALWAYS, "memblock: evict %v err %v", pid, err)
			err1 = err
			continue
		}
		if _, err := b.sc.WaitExit(pid); err != nil {
			db.DPrintf(db.ALWAYS, "memblock: waitexit %v err %v", pid, err)
			err1 = err
		}
	}
	db.DPrintf(db.ALWAYS, "memblock: evicted %v blockers", len(b.pids))
	b.pids = b.pids[:0]
	return err1
}

// Blocked is the kernel IDs that currently have a blocker, read from the
// directory memblock procs register themselves in. Sorted, for the same reason
// Kernels is.
//
// Any client with an FsLib can ask, which is what lets a proc that took no part
// in the blocking — the MR coordinator, say — find out which machines were set
// aside. Empty (or absent, before the first blocker has ever run) means no node
// is blocked.
func Blocked(fsl *fslib.FsLib) ([]string, error) {
	sts, err := fsl.GetDir(sp.MEMBLOCK)
	if err != nil {
		if serr.IsErrorNotfound(err) {
			return []string{}, nil
		}
		return nil, err
	}
	kids := sp.Names(sts)
	sort.Strings(kids)
	return kids, nil
}
