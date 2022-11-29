package lockmap

import (
	"strings"
	"sync"

	// "github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/sigmap"
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
	sync.Mutex
	//deadlock.Mutex
	path string // the locked path
}

func mkLock(p string) *PathLock {
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

func MkPathLockTable() *PathLockTable {
	plt := &PathLockTable{}
	plt.RefTable = refmap.MkRefTable[string, *PathLock]("LOCKMAP")
	return plt
}

// Caller must hold plt lock
func (plt *PathLockTable) allocLockStringL(pn string) *PathLock {
	sanitizedPn := strings.Trim(pn, "/")
	lk, _ := plt.Insert(sanitizedPn, func() *PathLock { return mkLock(sanitizedPn) })
	return lk
}

func (plt *PathLockTable) allocLock(path np.Path) *PathLock {
	plt.Lock()
	defer plt.Unlock()
	return plt.allocLockStringL(path.String())
}

func (plt *PathLockTable) allocLockString(pn string) *PathLock {
	plt.Lock()
	defer plt.Unlock()
	// Normalize paths (e.g., delete leading/trailing "/"s) so that matches
	// work for equivalent paths
	return plt.allocLockStringL(pn)
}

func (plt *PathLockTable) Acquire(ctx fs.CtxI, path np.Path) *PathLock {
	lk := plt.allocLock(path)
	lk.Lock()
	db.DPrintf("LOCKMAP", "%v: Lock '%s'", ctx.Uname(), lk.path)
	return lk
}

func (plt *PathLockTable) release(lk *PathLock) bool {
	plt.Lock()
	defer plt.Unlock()
	return plt.Delete(lk.path)
}

// Release lock for path. Caller should have watch locked through
// Acquire().
func (plt *PathLockTable) Release(ctx fs.CtxI, lk *PathLock) {
	db.DPrintf("LOCKMAP", "%v: Release '%s'", ctx.Uname(), lk.path)
	lk.Unlock()
	plt.release(lk)
}

// Caller must have dlk locked
func (plt *PathLockTable) HandOverLock(ctx fs.CtxI, dlk *PathLock, name string) *PathLock {
	flk := plt.allocLockString(dlk.path + "/" + name)

	db.DPrintf("LOCKMAP", "%v: HandoverLock '%s' %s", ctx.Uname(), dlk.path, name)

	flk.Lock()
	plt.Release(ctx, dlk)
	return flk
}

func (plt *PathLockTable) AcquireLocks(ctx fs.CtxI, dir np.Path, file string) (*PathLock, *PathLock) {
	dlk := plt.Acquire(ctx, dir)
	flk := plt.Acquire(ctx, append(dir, file))
	return dlk, flk
}

func (plt *PathLockTable) ReleaseLocks(ctx fs.CtxI, dlk, flk *PathLock) {
	plt.Release(ctx, dlk)
	plt.Release(ctx, flk)
}
