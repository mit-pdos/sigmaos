package fences1

import (
	"encoding/json"
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

//
// Map of fences indexed by pathname of fence at server.  Use by fssrv
// to keep track of the most recent fence seen.
//

type FenceTable struct {
	sync.Mutex
	fences map[np.Tpath]np.Tfence1
}

func MakeFenceTable() *FenceTable {
	ft := &FenceTable{}
	ft.fences = make(map[np.Tpath]np.Tfence1)
	return ft
}

// If no fence exists for this fence id, store it as most recent
// fence.  If the fence exists but newer, update the fence.  If the
// fence is stale, return error.  XXX check that clnt is allowed to
// update fence
func (ft *FenceTable) CheckFence(new np.Tfence1) (bool, *np.Err) {
	ft.Lock()
	defer ft.Unlock()

	update := false
	if new.FenceId.Path == 0 {
		return update, nil
	}
	p := new.FenceId.Path
	if f, ok := ft.fences[p]; ok {
		if new.Epoch < f.Epoch {
			db.DLPrintf("FENCES_ERR", "Stale fence %v\n", new)
			return update, np.MkErr(np.TErrStale, new)
		}
		if new.Epoch > f.Epoch {
			update = true
		}
	}
	ft.fences[p] = new
	return update, nil
}

func (ft *FenceTable) Snapshot() []byte {
	b, err := json.Marshal(ft.fences)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding fence table: %v", err)
	}
	return b
}

func RestoreRecentTable(b []byte) *FenceTable {
	ft := &FenceTable{}
	err := json.Unmarshal(b, &ft.fences)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fences in restore: %v", err)
	}
	return ft
}
