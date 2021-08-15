package sync

import (
	"log"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	DIR_LOCK  = "DIR_LOCK"   // Protects cond var directory
	BROADCAST = ".BROADCAST" // A file watched by everone for broadcasts
)

type Cond struct {
	condLock  *Lock  // Lock this condition variable protects
	dirLock   *Lock  // Lock protecting this condition variable (used to avoid sleep/wake races)
	path      string // Path to condition variable
	bcastPath string // Path to broadcast file watched by everyone
	*fslib.FsLib
}

// Strict lock checking is turned on if this is a true condition variable.
func MakeCond(fsl *fslib.FsLib, condpath string, lock *Lock) *Cond {
	c := &Cond{}
	c.condLock = lock
	c.path = condpath
	c.bcastPath = path.Join(condpath, BROADCAST)
	c.FsLib = fsl

	c.dirLock = MakeLock(fsl, fslib.LOCKS, fslib.LockName(path.Join(c.path, DIR_LOCK)), lock != nil)

	return c
}

// Initialize the condition variable by creating its sigmaOS state. This should
// only ever be called once globally per condition variable.
func (c *Cond) Init() error {
	// Make a directory in which waiters register themselves
	err := c.Mkdir(c.path, 0777)
	if err != nil {
		db.DLPrintf("COND", "Error condvar Init MkDir: %v", err)
		return err
	}

	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	c.createBcastFile()
	return nil
}

// Wake up all waiters. The condLock need not be held, and needs to be manually
// unlocked independently of this function call.
func (c *Cond) Broadcast() {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	err := c.Remove(c.bcastPath)
	if err != nil {
		log.Printf("Error Remove in Cond.Broadcast: %v", err)
	}

	c.createBcastFile()
}

// Wake up one waiter. The condLock need not be held, and needs to be manually
// unlocked independently of this function call..
func (c *Cond) Signal() {
	log.Fatalf("Cond.Signal unimplemented")
}

// Wait. If condLock != nil, assumes the condLock is held, and returns with the
// condLock held once again.
func (c *Cond) Wait() {
	c.dirLock.Lock()

	done := make(chan bool, 1)

	// Everyone waits on the broadcast file
	go func() {
		bcast := make(chan bool)
		err := c.SetRemoveWatch(c.bcastPath, func(p string, err error) {
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil {
				db.DLPrintf("COND", "Error RemoveWatch bcast triggered in Cond.Wait: %v", err)
			}
			bcast <- true
		})
		// If error, don't wait.
		if err == nil {
			<-bcast
		} else {
			db.DLPrintf("COND", "Error SetRemoveWatch bcast Cond.Wait: %v", err)
		}
		done <- true
	}()

	if c.condLock != nil {
		c.condLock.Unlock()
	}
	c.dirLock.Unlock()

	// Wait for either the Signal or Broadcast watch to be triggered
	<-done

	// Lock & return
	if c.condLock != nil {
		c.condLock.Lock()
	}
}

// Tear down a condition variable by waking all waiters and deleting the
// condition variable directory. Return the names of all the waiters. This will
// make waiting on it an error.
func (c *Cond) Destroy() []string {
	c.dirLock.Lock()

	waiterNames := []string{}

	// Wake up all waiters with a broadcast.
	err := c.Remove(c.bcastPath)
	if err != nil {
		log.Fatalf("Error Remove 1 in Cond.Destroy: %v", err)
	}

	// Remove the directory so we don't take on any more waiters
	err = c.Remove(c.path)
	if err != nil {
		log.Fatalf("Error Remove 2 in Cond.Destroy: %v", err)
	}

	c.dirLock.Unlock()

	// XXX Remove if tests pass...
	//	c.dirLock.Unlock()
	//
	//	// Rename the directory to make sure we don't take on any more waiters.
	//	newPath := path.Join(fslib.TMP, randstr.Hex(16))
	//	err = c.Rename(c.path, newPath)
	//	if err != nil {
	//		log.Fatalf("Error Rename in Cond.Destroy: %v", err)
	//	}
	//
	//	err = c.Remove(newPath)
	//	if err != nil {
	//		log.Fatalf("Error Remove 2 in Cond.Destroy: %v", err)
	//	}
	return waiterNames
}

// Make a broadcast file to be waited on.
func (c *Cond) createBcastFile() {
	err := c.MakeFile(c.bcastPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		log.Fatalf("Error condvar createBcastFile MakeFile: %v", err)
	}
}
