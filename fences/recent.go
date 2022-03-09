package fences

import (
	"sync"

	np "ulambda/ninep"
)

//
// Map of fences indexed by pathname of fence at server.  Use
// by fssrv to keep trac of the most recent fence seen.
//

type RecentTable struct {
	sync.Mutex
	fences map[string]np.Tfence
}

func MakeRecentTable() *RecentTable {
	rft := &RecentTable{}
	rft.fences = make(map[string]np.Tfence)
	return rft
}

// Return the most recent fence. If none, make it.
func (rft *RecentTable) MkFence(path np.Path) np.Tfence {
	rft.Lock()
	defer rft.Unlock()

	p := path.String()
	if f, ok := rft.fences[p]; ok {
		return f
	}

	idf := np.Tfenceid{p, 0}
	fence := np.Tfence{idf, 1}
	rft.fences[p] = fence

	return fence
}

// A new acquisition of a file or a modification of a file that may
// have a fence associated with it. If so, increase seqno of fence.
func (rft *RecentTable) UpdateSeqno(path np.Path) {
	rft.Lock()
	defer rft.Unlock()

	p := path.String()
	// log.Printf("%v: UpdateSeqno: fence %v\n", db.GetName(), p)
	if f, ok := rft.fences[p]; ok {
		f.Seqno += 1
		rft.fences[p] = f
	}
}

// If no fence exists for this fence id, store it as most recent
// fence.  If the fence exists but newer, update the fence.  If the
// fence is stale, return error.  XXX check that clnt is allowed to
// update fence (e.g., same user id as for existing fence?)
func (rft *RecentTable) UpdateFence(fence np.Tfence) *np.Err {
	rft.Lock()
	defer rft.Unlock()

	p := fence.FenceId.Path
	if f, ok := rft.fences[p]; ok {
		// log.Printf("%v: UpdateFence: fence %v new %v\n", db.GetName(), p, fence)
		if fence.Seqno < f.Seqno {
			return np.MkErr(np.TErrStale, p)
		}
	}
	rft.fences[p] = fence
	return nil
}

// Remove fence. Client better be sure that there no procs exists that
// rely on the fence.  XXX check that clnt is allowed to remove fence.
func (rft *RecentTable) RmFence(fence np.Tfence) *np.Err {
	rft.Lock()
	defer rft.Unlock()

	p := fence.FenceId.Path
	if f, ok := rft.fences[p]; ok {
		if fence.Seqno < f.Seqno {
			return np.MkErr(np.TErrStale, fence)
		}
		delete(rft.fences, p)
	} else {
		return np.MkErr(np.TErrUnknownFence, p)
	}
	return nil
}

// Check if supplied fence is recent.
func (rft *RecentTable) IsRecent(fence np.Tfence) *np.Err {
	rft.Lock()
	defer rft.Unlock()

	if f, ok := rft.fences[fence.FenceId.Path]; ok {
		if fence.Seqno < f.Seqno {
			return np.MkErr(np.TErrStale, fence)
		}
		return nil
	}
	return np.MkErr(np.TErrUnknownFence, fence.FenceId)
}
