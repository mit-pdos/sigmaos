package lockmap

import (
	"strings"
	"sync"

	// "github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/util/refmap"
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
func (plt *PathLockTable) allocLockStringL(sanitizedPN string) *PathLock {
	lk, _ := plt.Insert(sanitizedPN, func() *PathLock { return newLock(sanitizedPN) })
	return lk
}

func (plt *PathLockTable) allocLock(p path.Tpathname) *PathLock {
	sanitizedPN := strings.Trim(p.String(), "/")

	plt.Lock()
	defer plt.Unlock()

	return plt.allocLockStringL(sanitizedPN)
}

func (plt *PathLockTable) allocLockString(pn string) *PathLock {
	sanitizedPN := strings.Trim(pn, "/")

	plt.Lock()
	defer plt.Unlock()

	// Normalize paths (e.g., delete leading/trailing "/"s) so that matches
	// work for equivalent paths
	return plt.allocLockStringL(sanitizedPN)
}

func (plt *PathLockTable) Acquire(ctx fs.CtxI, path path.Tpathname, ltype Tlock) *PathLock {
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
func (plt *PathLockTable) HandOverLock(ctx fs.CtxI, dlk *PathLock, name string, ltype Tlock) *PathLock {
	flk := plt.allocLockString(dlk.path + "/" + name)

	db.DPrintf(db.LOCKMAP, "%v: HandoverLock '%s' %s", ctx.Principal(), dlk.path, name)

	if ltype == WLOCK {
		flk.Lock()
	} else {
		flk.RLock()
	}
	plt.Release(ctx, dlk, ltype)
	return flk
}

func (plt *PathLockTable) AcquireLocks(ctx fs.CtxI, dir path.Tpathname, file string, ltype Tlock) (*PathLock, *PathLock) {
	dlk := plt.Acquire(ctx, dir, ltype)
	flk := plt.Acquire(ctx, append(dir, file), ltype)
	return dlk, flk
}

func (plt *PathLockTable) ReleaseLocks(ctx fs.CtxI, dlk, flk *PathLock, ltype Tlock) {
	plt.Release(ctx, dlk, ltype)
	plt.Release(ctx, flk, ltype)
}
