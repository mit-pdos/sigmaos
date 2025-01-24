// Package lockmap provides table for locking pathnames.
// [spproto/srv] locks Tpath of a directory/file before manipulating
// it.  When a server starts an operation it calls Acquire, which
// allocates a pathlock in the table and locks the pathlock. Then, it
// does it work, and releases the pathlock at the end.  If the
// releasing thread is the last thread using the pathlock, then the
// thread removes the pathlock from the table.
package lockmap

import (
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/refmap"
)

type Tlock int

const (
	RLOCK Tlock = iota + 1
	WLOCK

	N = 1000
)

func (t Tlock) String() string {
	switch t {
	case RLOCK:
		return "RLock"
	case WLOCK:
		return "WLock"
	default:
		db.DFatalf("Unknown lock type %v", int(t))
		return "unknown"
	}
}

type PathLock struct {
	sync.RWMutex
	path sp.Tpath
}

func (pl *PathLock) Path() sp.Tpath {
	return pl.path
}

func newLock(p sp.Tpath) PathLock {
	return PathLock{path: p}
}

type PathLockTable struct {
	//	deadlock.Mutex
	sync.Mutex
	*refmap.RefTable[sp.Tpath, PathLock]
}

func NewPathLockTable() *PathLockTable {
	plt := &PathLockTable{}
	plt.RefTable = refmap.NewRefTable[sp.Tpath, PathLock](N, db.LOCKMAP)
	return plt
}

func (plt *PathLockTable) Len() (int, int) {
	plt.Lock()
	defer plt.Unlock()
	return plt.RefTable.Len()
}

// Caller must hold plt lock
func (plt *PathLockTable) allocLockL(p sp.Tpath) *PathLock {
	lk, _ := plt.Insert(p, newLock(p))
	return lk
}

func (plt *PathLockTable) allocLock(p sp.Tpath) *PathLock {
	plt.Lock()
	defer plt.Unlock()
	return plt.allocLockL(p)
}

func (plt *PathLockTable) Acquire(ctx fs.CtxI, path sp.Tpath, ltype Tlock) *PathLock {
	lk := plt.allocLock(path)
	if ltype == WLOCK {
		lk.Lock()
	} else {
		lk.RLock()
	}
	db.DPrintf(db.LOCKMAP, "%v: Lock %v", ctx.Principal(), lk.path)
	return lk
}

func (plt *PathLockTable) release(lk *PathLock) (bool, error) {
	plt.Lock()
	defer plt.Unlock()
	return plt.Delete(lk.path)
}

// Release lock for path. Caller should have watch locked through
// Acquire().
func (plt *PathLockTable) Release(ctx fs.CtxI, lk *PathLock, ltype Tlock) {
	db.DPrintf(db.LOCKMAP, "%v: Release %v", ctx.Principal(), lk.path)
	if ltype == WLOCK {
		lk.Unlock()
	} else {
		lk.RUnlock()
	}
	plt.release(lk)
}

// Caller must have dlk locked
func (plt *PathLockTable) HandOverLock(ctx fs.CtxI, dlk *PathLock, path sp.Tpath, ltype Tlock) *PathLock {
	flk := plt.allocLock(path)

	db.DPrintf(db.LOCKMAP, "%v: HandoverLock %v %v", ctx.Principal(), dlk.path, path)

	if ltype == WLOCK {
		flk.Lock()
	} else {
		flk.RLock()
	}
	plt.Release(ctx, dlk, ltype)
	return flk
}
