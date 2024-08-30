package procclnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// All the state a procclnt holds about its children.
type ChildState struct {
	sync.Mutex
	ranOn      map[sp.Tpid]*SpawnFuture
	exitStatus map[sp.Tpid]*proc.Status
}

func newChildState() *ChildState {
	return &ChildState{
		ranOn:      make(map[sp.Tpid]*SpawnFuture),
		exitStatus: make(map[sp.Tpid]*proc.Status),
	}
}

type SpawnFuture struct {
	seqno *proc.ProcSeqno
	err   error
	cond  *sync.Cond
	done  bool
}

func newSpawnFuture(mu sync.Locker) *SpawnFuture {
	return &SpawnFuture{
		cond: sync.NewCond(mu),
		done: false,
	}
}

// Caller holds lock
func (sf *SpawnFuture) Get() (*proc.ProcSeqno, error) {
	if !sf.done {
		sf.cond.Wait()
		// Sanity check that the value has materalized.
		if !sf.done {
			db.DFatalf("Err wait condition false")
		}
	}
	return sf.seqno, sf.err
}

// Caller holds lock.
func (sf *SpawnFuture) Complete(seqno *proc.ProcSeqno, err error) {
	// Sanity check that completions only happen once.
	if sf.done {
		db.DPrintf(db.ERROR, "Double-completed spawn future %v", seqno)
		db.DFatalf("Double-completed spawn future %v", seqno)
	}
	sf.seqno = seqno
	sf.err = err
	sf.done = true
	sf.cond.Broadcast()
}

func (cs *ChildState) Spawned(pid sp.Tpid) {
	cs.Lock()
	defer cs.Unlock()

	// Record ID of schedd this proc was spawned on
	cs.ranOn[pid] = newSpawnFuture(&cs.Mutex)
}

func (cs *ChildState) Started(pid sp.Tpid, seqno *proc.ProcSeqno, err error) {
	cs.Lock()
	defer cs.Unlock()

	// Record ID of schedd this proc was spawned on
	if sf, ok := cs.ranOn[pid]; ok {
		sf.Complete(seqno, err)
	} else {
		db.DPrintf(db.ERROR, "Error started unknown proc")
	}
}

func (cs *ChildState) Exited(pid sp.Tpid, status *proc.Status) {
	cs.Lock()
	defer cs.Unlock()

	// Sanity check that threads won't block indefinitely.
	if !cs.ranOn[pid].done {
		db.DPrintf(db.ERROR, "Error exited future not completed %v status %v", pid, status)
		db.DFatalf("Error exited future not completed %v status %v", pid, status)
	}
	// Clean up child state
	delete(cs.ranOn, pid)
	cs.exitStatus[pid] = status
}

func (cs *ChildState) GetExitStatus(pid sp.Tpid) *proc.Status {
	cs.Lock()
	defer cs.Unlock()

	status, ok := cs.exitStatus[pid]
	if !ok {
		db.DFatalf("Try to get exit status for proc which never exited")
	}
	delete(cs.exitStatus, pid)
	return status
}

func (cs *ChildState) GetProcSeqno(pid sp.Tpid) (*proc.ProcSeqno, error) {
	cs.Lock()
	defer cs.Unlock()

	// If the proc already ran, return the ID of the schedd it ran on.
	if fut, ok := cs.ranOn[pid]; ok {
		return fut.Get()
	}
	return nil, fmt.Errorf("Proc %v child state not found", pid)
}
