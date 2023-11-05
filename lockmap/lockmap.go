package lockmap

import (
	"strings"
	"sync"

	// "github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/refmap"
)

//
// A table for locking pathnames.  Servers lock a pathname before
// manipulating/creating a file or directory.  When a server starts an
// operation it calls Acquire, which allocates a pathlock in the table
// and locks the pathlock. Then, it does it work, and releases the
// pathlock at the end.  If the releasing thread is the last thread
// using the pathlock, then the thread removes the pathlock from the
// table.  Thread acquire path locks in the following order: first the
// parent directory, then the file or child directory.
//

type PathLock struct {
	sync.RWMutex
	//deadlock.Mutex
	path string // the locked path
}

func newLock(p string) *PathLock {
	lk := &PathLock{}
	lk.path = p
	return lk
}

func (pl *PathLock) Path() string {
	return pl.path
}

type PathLockTable struct {
	//	deadlock.Mutex
	sync.Mutex
	*refmap.RefTable[string, *PathLock]
}

func NewPathLockTable() *PathLockTable {
	plt := &PathLockTable{}
	plt.RefTable = refmap.NewRefTable[string, *PathLock](db.LOCKMAP)
	return plt
}

// Caller must hold plt lock
func (plt *PathLockTable) allocLockStringL(pn string) *PathLock {
	sanitizedPn := strings.Trim(pn, "/")
	lk, _ := plt.Insert(sanitizedPn, func() *PathLock { return newLock(sanitizedPn) })
	return lk
}

func (plt *PathLockTable) allocLock(p path.Path) *PathLock {
	plt.Lock()
	defer plt.Unlock()
	return plt.allocLockStringL(p.String())
}

func (plt *PathLockTable) allocLockString(pn string) *PathLock {
	plt.Lock()
	defer plt.Unlock()
	// Normalize paths (e.g., delete leading/trailing "/"s) so that matches
	// work for equivalent paths
	return plt.allocLockStringL(pn)
}

func (plt *PathLockTable) Acquire(ctx fs.CtxI, path path.Path, write bool) *PathLock {
	lk := plt.allocLock(path)
	if write {
		lk.Lock()
	} else {
		lk.RLock()
	}
	db.DPrintf(db.LOCKMAP, "%v: Lock '%s'", ctx.Uname(), lk.path)
	return lk
}

func (plt *PathLockTable) release(lk *PathLock) bool {
	plt.Lock()
	defer plt.Unlock()
	return plt.Delete(lk.path)
}

// Release lock for path. Caller should have watch locked through
// Acquire().
func (plt *PathLockTable) Release(ctx fs.CtxI, lk *PathLock, write bool) {
	db.DPrintf(db.LOCKMAP, "%v: Release '%s'", ctx.Uname(), lk.path)
	if write {
		lk.Unlock()
	} else {
		lk.RUnlock()
	}
	plt.release(lk)
}

// Caller must have dlk locked
func (plt *PathLockTable) HandOverLock(ctx fs.CtxI, dlk *PathLock, name string, write bool) *PathLock {
	flk := plt.allocLockString(dlk.path + "/" + name)

	db.DPrintf(db.LOCKMAP, "%v: HandoverLock '%s' %s", ctx.Uname(), dlk.path, name)

	if write {
		flk.Lock()
	} else {
		flk.RLock()
	}
	plt.Release(ctx, dlk, write)
	return flk
}

func (plt *PathLockTable) AcquireLocks(ctx fs.CtxI, dir path.Path, file string, write bool) (*PathLock, *PathLock) {
	dlk := plt.Acquire(ctx, dir, write)
	flk := plt.Acquire(ctx, append(dir, file), write)
	return dlk, flk
}

func (plt *PathLockTable) ReleaseLocks(ctx fs.CtxI, dlk, flk *PathLock, write bool) {
	plt.Release(ctx, dlk, write)
	plt.Release(ctx, flk, write)
}
