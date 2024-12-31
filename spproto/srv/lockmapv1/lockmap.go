// Package lockmap provides table for locking pathnames.  Servers lock
// a pathname before manipulating/creating a file or directory.  When
// a server starts an operation it calls Acquire, which allocates a
// pathlock in the table and locks the pathlock. Then, it does it
// work, and releases the pathlock at the end.  If the releasing
// thread is the last thread using the pathlock, then the thread
// removes the pathlock from the table.  Thread acquire path locks in
// the following order: first the parent directory, then the file or
// child directory.
//

package lockmapv1

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

func newLock(p sp.Tpath) *PathLock {
	return &PathLock{path: p}
}

type PathLockTable struct {
	//	deadlock.Mutex
	sync.Mutex
	*refmap.RefTable[sp.Tpath, *PathLock]
}

func NewPathLockTable() *PathLockTable {
	plt := &PathLockTable{}
	plt.RefTable = refmap.NewRefTable[sp.Tpath, *PathLock](db.LOCKMAP)
	return plt
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
	db.DPrintf(db.LOCKMAP, "%v: Lock '%s'", ctx.Principal(), lk.path)
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
	db.DPrintf(db.LOCKMAP, "%v: Release '%s'", ctx.Principal(), lk.path)
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

	db.DPrintf(db.LOCKMAP, "%v: HandoverLock '%s' %s", ctx.Principal(), dlk.path, path)

	if ltype == WLOCK {
		flk.Lock()
	} else {
		flk.RLock()
	}
	plt.Release(ctx, dlk, ltype)
	return flk
}
