package procclnt

import (
	"fmt"
	"sync"

	"golang.org/x/exp/slices" // todo: upgrade to > 1.21

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

// All the state a procclnt holds about its children.
type ChildState struct {
	sync.Mutex
	ranOn      map[sp.Tpid]*SpawnFuture
	exitStatus map[sp.Tpid]*proc.Status
	bins       map[string][]string // map from bin name to kernelIdy
}

func newChildState() *ChildState {
	return &ChildState{
		ranOn:      make(map[sp.Tpid]*SpawnFuture),
		exitStatus: make(map[sp.Tpid]*proc.Status),
		bins:       make(map[string][]string),
	}
}

type SpawnFuture struct {
	kernelID string
	err      error
	cond     *sync.Cond
	done     bool
}

func newSpawnFuture(mu sync.Locker) *SpawnFuture {
	return &SpawnFuture{
		cond: sync.NewCond(mu),
		done: false,
	}
}

// Caller holds lock
func (sf *SpawnFuture) Get() (string, error) {
	if !sf.done {
		sf.cond.Wait()
		// Sanity check that the value has materalized.
		if !sf.done {
			db.DFatalf("Err wait condition false")
		}
	}
	return sf.kernelID, sf.err
}

// Caller holds lock.
func (sf *SpawnFuture) Complete(kernelID string, err error) {
	// Sanity check that completions only happen once.
	if sf.done {
		db.DFatalf("Double-completed spawn future")
	}
	sf.kernelID = kernelID
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

func (cs *ChildState) Started(pid sp.Tpid, kernelID string, err error) {
	cs.Lock()
	defer cs.Unlock()

	// Record ID of schedd this proc was spawned on
	cs.ranOn[pid].Complete(kernelID, err)
}

func (cs *ChildState) Exited(pid sp.Tpid, status *proc.Status) {
	cs.Lock()
	defer cs.Unlock()

	// Sanity check that threads won't block indefinitely.
	if !cs.ranOn[pid].done {
		db.DFatalf("Error exited future not completed")
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

func (cs *ChildState) GetKernelID(pid sp.Tpid) (string, error) {
	cs.Lock()
	defer cs.Unlock()

	// If the proc already ran, return the ID of the schedd it ran on.
	if fut, ok := cs.ranOn[pid]; ok {
		return fut.Get()
	}
	return "NO_SCHEDD", fmt.Errorf("Proc %v child state not found", pid)
}

func (cs *ChildState) GetBinKernelID(bin string) (string, bool) {
	cs.Lock()
	defer cs.Unlock()

	if kids, ok := cs.bins[bin]; ok {
		i := rand.Int64(int64(len(kids)))
		return kids[int(i)], true
	}
	return "", false
}

func (cs *ChildState) SetBinKernelID(bin, kernelId string) {
	cs.Lock()
	defer cs.Unlock()

	if _, ok := cs.bins[bin]; ok {
		cs.bins[bin] = append(cs.bins[bin], kernelId)
	} else {
		cs.bins[bin] = []string{kernelId}
	}
}

func (cs *ChildState) DelBinKernelID(bin, kernelId string) {
	cs.Lock()
	defer cs.Unlock()

	if _, ok := cs.bins[bin]; ok {
		i := slices.IndexFunc(cs.bins[bin], func(s string) bool { return s == kernelId })
		cs.bins[bin] = append(cs.bins[bin][:i], cs.bins[bin][i+1:]...)
	}
}
