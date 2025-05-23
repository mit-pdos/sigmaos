// The semaphore package implements a counting semaphore, implemented
// using a file and a watch.
package semaphore

import (
	"fmt"
	"time"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dirwatcher"
	sp "sigmaos/sigmap"
)

type Semaphore struct {
	path string // Path for semaphore variable
	*fslib.FsLib
}

func NewSemaphore(fsl *fslib.FsLib, semaphore string) *Semaphore {
	sem := &Semaphore{}
	sem.path = semaphore
	sem.FsLib = fsl
	return sem
}

// Initialize semaphore variable by creating its sigmaOS state. This should
// only ever be called once globally.
func (sem *Semaphore) Init(perm sp.Tperm) error {
	db.DPrintf(db.SEMCLNT, "Semaphore init %v\n", sem.path)
	_, err := sem.PutFile(sem.path, 0777|perm, sp.OWRITE, []byte{})
	return err
}

func (sem *Semaphore) InitLease(perm sp.Tperm, lid sp.TleaseId) error {
	db.DPrintf(db.SEMCLNT, "Semaphore init %v lease %v\n", sem.path, lid)
	_, err := sem.PutLeasedFile(sem.path, 0777|perm, sp.OWRITE, lid, []byte{})
	return err
}

// Down semaphore. If not upped yet (i.e., if file exists), block
func (sem *Semaphore) Down() error {
	for i := 0; i < sp.Conf.Path.MAX_RESOLVE_RETRY; i++ {
		db.DPrintf(db.SEMCLNT, "Down %d %v\n", i, sem.path)
		err := dirwatcher.WaitRemove(sem.FsLib, sem.path)
		// If err is because file has been removed, then no error: the
		// semaphore has been "upped".
		if serr.IsErrCode(err, serr.TErrNotfound) {
			db.DPrintf(db.SEMCLNT_ERR, "down notfound %v ok err %v\n", sem.path, err)
			return nil
		}
		if serr.IsErrorUnreachable(err) || serr.IsErrCode(err, serr.TErrClosed) {
			db.DPrintf(db.SEMCLNT, "down unreachable %v ok err %v\n", sem.path, err)
			time.Sleep(sp.Conf.Path.RESOLVE_TIMEOUT)
			continue
		}
		if err == nil {
			db.DPrintf(db.SEMCLNT, "semaphore done wait %v\n", sem.path)
			return nil
		} else {
			db.DPrintf(db.SEMCLNT_ERR, "down %v err %v\n", sem.path, err)
			return err
		}
	}
	db.DPrintf(db.SEMCLNT_ERR, "Down failed after %d retries\n", sp.Conf.Path.MAX_RESOLVE_RETRY)
	return serr.NewErr(serr.TErrUnreachable, sem.path)
}

// Up a semaphore variable (i.e., remove semaphore to indicate up has
// happened).
func (sem *Semaphore) Up() error {
	db.DPrintf(db.SEMCLNT, "Up %v\n", sem.path)
	return sem.Remove(sem.path)
}

func (sem *Semaphore) GetPath() string {
	return sem.path
}

func (sem *Semaphore) String() string {
	return fmt.Sprintf("sem %v", sem.path)
}
