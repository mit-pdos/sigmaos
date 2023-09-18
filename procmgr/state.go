package procmgr

import (
	"sync"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type ProcState struct {
	sync.Mutex
	spawned     map[sp.Tpid]*proc.Proc
	startWaiter map[sp.Tpid]*Waiter
	evictWaiter map[sp.Tpid]*Waiter
	exitWaiter  map[sp.Tpid]*Waiter
}

func NewProcState() *ProcState {
	return &ProcState{
		spawned:     make(map[sp.Tpid]*proc.Proc),
		startWaiter: make(map[sp.Tpid]*Waiter),
		evictWaiter: make(map[sp.Tpid]*Waiter),
		exitWaiter:  make(map[sp.Tpid]*Waiter),
	}
}

func (ps *ProcState) spawn(p *proc.Proc) {
	ps.Lock()
	defer ps.Unlock()

	ps.spawned[p.GetPid()] = p
	ps.startWaiter[p.GetPid()] = newWaiter(&ps.Mutex)
	ps.evictWaiter[p.GetPid()] = newWaiter(&ps.Mutex)
	ps.exitWaiter[p.GetPid()] = newWaiter(&ps.Mutex)
}

func (ps *ProcState) started(pid sp.Tpid) {
	ps.Lock()
	defer ps.Unlock()

	ps.startWaiter[pid].release()
}

func (ps *ProcState) waitStart(pid sp.Tpid) {
	ps.Lock()
	defer ps.Unlock()

	if w, ok := ps.startWaiter[pid]; ok {
		w.wait()
	}
}

func (ps *ProcState) evicted(pid sp.Tpid) {
	ps.Lock()
	defer ps.Unlock()

	ps.evictWaiter[pid].release()
}

func (ps *ProcState) waitEvict(pid sp.Tpid) {
	ps.Lock()
	defer ps.Unlock()

	if w, ok := ps.evictWaiter[pid]; ok {
		w.wait()
	}
}

func (ps *ProcState) exited(pid sp.Tpid) {
	ps.Lock()
	defer ps.Unlock()

	ps.exitWaiter[pid].release()
	// Clean up state
	delete(ps.spawned, pid)
	delete(ps.startWaiter, pid)
	delete(ps.evictWaiter, pid)
	delete(ps.exitWaiter, pid)
}

func (ps *ProcState) waitExit(pid sp.Tpid) {
	ps.Lock()
	defer ps.Unlock()

	// If proc exited already, the waiter may no longer be present.
	if w, ok := ps.exitWaiter[pid]; ok {
		w.wait()
	}
}
