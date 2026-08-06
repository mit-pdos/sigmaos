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
// # A claim, not an allocation
//
// Nothing here consumes memory. A node stops accepting procs when msched's memory
// budget — the figure besched filters candidate nodes on — drops below what a proc
// requests, and that budget is accounting: a blocker claims it with Proc.SetMem and
// holds the claim by staying alive.
//
// Which is what we want. Actually consuming a machine's memory would evict the page
// cache of whatever server it was set aside to run, and risk the OOM killer taking
// it — the opposite of the point.
package memblock

import (
	"fmt"
	"path/filepath"
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
	maxRounds int
	pids      []sp.Tpid
	// Kernels this Blocker put blockers on, so that their markers can be taken away
	// again when the blocks are released.
	kids []string
}

type Opt func(*Blocker)

// WithMaxRounds sets how many blockers to try per node before reporting failure.
func WithMaxRounds(n int) Opt {
	return func(b *Blocker) { b.maxRounds = n }
}

func NewBlocker(sc *sigmaclnt.SigmaClnt, opts ...Opt) *Blocker {
	b := &Blocker{
		sc:        sc,
		msc:       mschedclnt.NewMSchedClnt(sc.FsLib, sp.NOT_SET),
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
	// A blocker that never came up leaves its kernel's marker behind, since
	// memblock writes that before it allocates. Take it away: a marker means "this
	// node is dedicated", and leaving a lie here is worse than failing, because a
	// caller reading the directory (the MR coordinator does) would send it work.
	b.unregister(kid)
	free, _, err := b.msc.GetMem(kid)
	return fmt.Errorf("memblock: %v still admits procs requesting %vMB after %v blockers (memFree %vMB, err %v); "+
		"the usual cause is the blockers exiting — a memblock that dies gives its memory straight back, so check its proc log",
		kid, reject, b.maxRounds, free, err)
}

// unregister removes a kernel's marker. Best-effort: it is cleanup, and a marker
// that can't be removed is reported by whoever next compares the directory against
// what they blocked.
func (b *Blocker) unregister(kid string) {
	pn := filepath.Join(sp.MEMBLOCK, kid)
	if err := b.sc.Remove(pn); err != nil {
		db.DPrintf(db.ALWAYS, "memblock: remove marker %v err %v", pn, err)
	}
}

// BlockAmount claims exactly mem MB of each of kids' budgets, rather than however
// much it takes to make the node refuse work.
//
// This is the other reason to block memory: not to stop procs from being placed at
// all, but to run an experiment against nodes that appear to the scheduler to have
// less memory than they do — the packing sweeps in the benchmarks do this.
func (b *Blocker) BlockAmount(kids []string, mem proc.Tmem) error {
	db.DPrintf(db.ALWAYS, "memblock: claiming %vMB on %v kernels %v", mem, len(kids), kids)
	for _, kid := range kids {
		if err := b.spawn(kid, mem); err != nil {
			return err
		}
	}
	return nil
}

// spawn starts one blocker pinned to kid, reserving mem, and waits for it to be
// running.
//
// WaitStart returning is not proof that it is: it also returns when a proc dies
// before starting, and a blocker that dies gives its memory straight back. Which
// is why the caller's loop re-reads memFree rather than counting blockers — that
// check, not this one, is what says a node is blocked.
func (b *Blocker) spawn(kid string, mem proc.Tmem) error {
	p := proc.NewProc(PROGRAM, nil)
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
	if len(b.kids) == 0 || b.kids[len(b.kids)-1] != kid {
		b.kids = append(b.kids, kid)
	}
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
	// The markers go with them: memblock writes one per kernel and never removes it,
	// so leaving them would tell the next reader that these machines are still
	// dedicated.
	for _, kid := range b.kids {
		b.unregister(kid)
	}
	db.DPrintf(db.ALWAYS, "memblock: evicted %v blockers on %v kernels", len(b.pids), len(b.kids))
	b.pids = b.pids[:0]
	b.kids = b.kids[:0]
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
