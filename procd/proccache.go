package procd

import (
	"sort"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
)

type ProcCacheEntry struct {
	lastUsed time.Time
	p        *proc.Proc
}

// Cache of proc structs which have been read by this procd.
type ProcCache struct {
	sync.Mutex
	maxSize int
	ps      map[proc.Tpid]*ProcCacheEntry
}

func MakeProcCache(maxSize int) *ProcCache {
	return &ProcCache{
		maxSize: maxSize,
		ps:      make(map[proc.Tpid]*ProcCacheEntry),
	}
}

func (pc *ProcCache) Get(pid proc.Tpid) (*proc.Proc, bool) {
	pc.Lock()
	defer pc.Unlock()

	pc.gc()

	if e, ok := pc.ps[pid]; ok {
		e.lastUsed = time.Now()
		return e.p, true
	}
	return nil, false
}

func (pc *ProcCache) Set(pid proc.Tpid, p *proc.Proc) {
	pc.Lock()
	defer pc.Unlock()

	pc.gc()

	if e, ok := pc.ps[pid]; ok {
		e.lastUsed = time.Now()
		return
	}
	pc.ps[pid] = &ProcCacheEntry{time.Now(), p}
}

func (pc *ProcCache) Remove(pid proc.Tpid) {
	pc.Lock()
	defer pc.Unlock()

	pc.gc()

	delete(pc.ps, pid)
}

// If there are too many procs in the cache, clean some of them out according
// to LRU.
// XXX Make more efficient?
func (pc *ProcCache) gc() {
	// If there aren't too many entries, return.
	if len(pc.ps) < pc.maxSize {
		return
	}
	db.DPrintf(db.PROCCACHE, "Doing GC")
	ps := make([]*ProcCacheEntry, 0, len(pc.ps))
	for _, pce := range pc.ps {
		ps = append(ps, pce)
	}
	// Sort according to LRU
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].lastUsed.UnixMicro() < ps[j].lastUsed.UnixMicro()
	})
	// Kill half of the entries.
	for i := 0; i < len(ps)/2; i++ {
		delete(pc.ps, ps[i].p.GetPid())
	}
}
