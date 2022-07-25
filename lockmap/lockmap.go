package lockmap

import (
	"sync"

	// "github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
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
	nref int    // updated under table lock
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
	locks map[string]*PathLock
}

func MkPathLockTable() *PathLockTable {
	plt := &PathLockTable{}
	plt.locks = make(map[string]*PathLock)
	return plt
}

// Caller must hold plt lock
func (plt *PathLockTable) allocLockStringL(pn string) *PathLock {
	lk, ok := plt.locks[pn]
	if !ok {
		db.DPrintf("LOCKMAP", "allocLock '%s'\n", pn)
		lk = mkLock(pn)
		plt.locks[pn] = lk
	}
	lk.nref++ // ensure ws won't be deleted from table
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
	return plt.allocLockStringL(pn)
}

// XXX Normalize paths (e.g., delete extra /) so that matches
// work for equivalent paths
func (plt *PathLockTable) Acquire(path np.Path) *PathLock {
	lk := plt.allocLock(path)
	lk.Lock()
	db.DPrintf("LOCKMAP", "Lock '%s'\n", lk.path)
	return lk
}

func (plt *PathLockTable) release(lk *PathLock) bool {
	plt.Lock()
	defer plt.Unlock()

	del := false
	lk.nref--

	lk1, ok := plt.locks[lk.path]
	if !ok {
		// Another thread already deleted the entry
		db.DFatalf("release '%v'\n", lk)
		return del
	}

	if lk != lk1 {
		db.DFatalf("Release\n")
	}

	if lk.nref == 0 {
		delete(plt.locks, lk.path)
		del = true
	}
	return del
}

// Release watch for path. Caller should have watch locked through
// Acquire().
func (plt *PathLockTable) Release(lk *PathLock) {
	db.DPrintf("LOCKMAP", "Release '%s'\n", lk.path)
	lk.Unlock()
	plt.release(lk)
}

// Caller must have dlk locked
func (plt *PathLockTable) HandOverLock(dlk *PathLock, name string) *PathLock {
	flk := plt.allocLockString(dlk.path + "/" + name)

	db.DPrintf("LOCKMAP", "HandoverLock '%s' %s\n", dlk.path, name)

	flk.Lock()
	plt.Release(dlk)
	return flk
}

func (plt *PathLockTable) AcquireLocks(dir np.Path, file string) (*PathLock, *PathLock) {
	dlk := plt.Acquire(dir)
	flk := plt.Acquire(append(dir, file))
	return dlk, flk
}

func (plt *PathLockTable) ReleaseLocks(dlk, flk *PathLock) {
	plt.Release(dlk)
	plt.Release(flk)
}
