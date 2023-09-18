package procclnt

import (
	"fmt"
	"sync"

	sp "sigmaos/sigmap"
)

// All the state a procclnt holds about its children.
type ChildState struct {
	sync.Mutex
	spawnedOn map[sp.Tpid]string
	ranOn     map[sp.Tpid]string
}

func newChildState() *ChildState {
	return &ChildState{
		spawnedOn: make(map[sp.Tpid]string),
		ranOn:     make(map[sp.Tpid]string),
	}
}

func (cs *ChildState) spawned(pid sp.Tpid, kernelID string) {
	cs.Lock()
	defer cs.Unlock()

	// Record ID of schedd this proc was spawned on
	cs.spawnedOn[pid] = kernelID
}

func (cs *ChildState) started(pid sp.Tpid, kernelID string) {
	cs.Lock()
	defer cs.Unlock()

	// Record ID of schedd this proc was spawned on
	cs.spawnedOn[pid] = kernelID
}

func (cs *ChildState) exited(pid sp.Tpid) {
	cs.Lock()
	defer cs.Unlock()

	// Clean up child state
	delete(cs.spawnedOn, pid)
	delete(cs.ranOn, pid)
}

func (cs *ChildState) getKernelID(pid sp.Tpid) (string, error) {
	cs.Lock()
	defer cs.Unlock()

	// If the proc already ran, return the ID of the schedd it ran on.
	if id, ok := cs.ranOn[pid]; ok {
		return id, nil
	}
	// Return the ID of the schedd the proc was spawned on.
	if id, ok := cs.spawnedOn[pid]; ok {
		return id, nil
	}
	return "NO_SCHEDD", fmt.Errorf("Proc %v child state not found", pid)
}
