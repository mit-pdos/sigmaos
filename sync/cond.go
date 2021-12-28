package sync

import (
	"log"
	"path"
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/rand"
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
	strict    bool   // If true, kill on error. Otherwise, print to debug
	*fslib.FsLib
}

// Strict lock checking is turned on if this is a true condition variable.
func MakeCond(fsl *fslib.FsLib, condpath string, lock *Lock, strict bool) *Cond {
	c := &Cond{}
	c.condLock = lock
	c.path = condpath
	c.bcastPath = path.Join(condpath, BROADCAST)
	c.strict = strict
	c.FsLib = fsl

	c.dirLock = MakeLock(fsl, np.LOCKS, path.Join(c.path, DIR_LOCK), lock != nil)

	return c
}

// Strict lock checking is turned on if this is a true condition variable.
func MakeCondNew(fsl *fslib.FsLib, dir string, cond string, lock *Lock) *Cond {
	c := &Cond{}
	c.condLock = lock
	c.path = path.Join(dir, cond)
	c.bcastPath = path.Join(c.path, BROADCAST)
	c.FsLib = fsl

	c.dirLock = MakeLock(fsl, dir, path.Join(c.path, DIR_LOCK), lock != nil)

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
		if c.strict {
			log.Fatalf("Error Remove in Cond.Broadcast: %v", err)
		} else {
			log.Printf("Error Remove in Cond.Broadcast: %v", err)
		}
	}

	c.createBcastFile()
}

// Wake up one waiter. The condLock need not be held, and needs to be manually
// unlocked independently of this function call..
func (c *Cond) Signal() {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	wFiles, err := c.ReadDir(c.path)
	if err != nil {
		debug.PrintStack()
		if c.strict {
			log.Printf("Error ReadDir %v in Cond.Signal: %v", c.path, err)
		} else {
			log.Fatalf("Error ReadDir %v in Cond.Signal: %v", c.path, err)
		}
	}

	var waiter string
	for {
		// If no one was waiting, return
		if len(wFiles) == 0 {
			return
		}
		i := rand.Int64(int64(len(wFiles)))
		waiter = wFiles[i].Name
		// Make sure we don't remove the broadcast file accidentally
		if waiter == BROADCAST {
			wFiles = append(wFiles[:i], wFiles[i+1:]...)
		} else {
			break
		}
	}

	err = c.Remove(path.Join(c.path, waiter))
	if err != nil {
		if c.strict {
			log.Printf("Error Remove in Cond.Signal: %v", err)
		} else {
			log.Fatalf("Error Remove in Cond.Signal: %v", err)
		}
	}
}

// Wait. If condLock != nil, assumes the condLock is held, and returns with the
// condLock held once again.
func (c *Cond) Wait() error {
	c.dirLock.Lock()

	done := make(chan error, 2)

	signalPath, err := c.createSignalFile()
	if err != nil {
		c.dirLock.Unlock()
		return err
	}

	// Everyone waits on the broadcast file
	go func() {
		bcast := make(chan error)
		err := c.SetRemoveWatch(c.bcastPath, func(p string, err error) {
			if err != nil {
				db.DLPrintf("COND", "Error RemoveWatch bcast triggered in Cond.Wait: %v", err)
			}
			bcast <- err
		})
		// If error, don't wait.
		if err == nil {
			err = <-bcast
		} else {
			db.DLPrintf("COND", "Error SetRemoveWatch bcast Cond.Wait: %v", err)
		}

		c.Remove(signalPath)
		done <- err
	}()

	// Each waiter waits on its own signal file
	go func() {
		signal := make(chan error)
		err := c.SetRemoveWatch(signalPath, func(p string, err error) {
			if err != nil {
				db.DLPrintf("COND", "Error RemoveWatch bcast triggered in Cond.Wait: %v", err)
			}
			signal <- err
		})
		// If error, don't wait.
		if err == nil {
			err = <-signal
		} else {
			db.DLPrintf("COND", "Error SetRemoveWatch bcast Cond.Wait: %v", err)
		}
		done <- err
	}()

	if c.condLock != nil {
		c.condLock.Unlock()
	}
	c.dirLock.Unlock()

	// Wait for either the Signal or Broadcast watch to be triggered
	err = <-done

	if err != nil {
		return err
	}

	// Lock & return
	if c.condLock != nil {
		c.condLock.Lock()
	}
	return nil
}

// Tear down a condition variable by waking all waiters and deleting the
// condition variable directory. Return the names of all the waiters. This will
// make waiting on it an error.
func (c *Cond) Destroy() {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	// Wake up all waiters with a broadcast.
	err := c.Remove(c.bcastPath)
	if err != nil {
		debug.PrintStack()
		if err.Error() == "EOF" {
			log.Printf("Error Remove 1 in Cond.Destroy: %v", err)
		} else {
			if c.strict {
				log.Fatalf("Error Remove 1 in Cond.Destroy: %v", err)
			} else {
				log.Printf("Error Remove 1 in Cond.Destroy: %v", err)
			}
		}
		return
	}

	// Remove the directory so we don't take on any more waiters
	err = c.RmDir(c.path)
	if err != nil {
		if err.Error() == "EOF" {
			log.Printf("Error Remove 2 in Cond.Destroy: %v", err)
		} else {
			if c.strict {
				log.Fatalf("Error Remove 2 in Cond.Destroy: %v", err)
			} else {
				log.Printf("Error Remove 2 in Cond.Destroy: %v", err)
			}
		}
		return
	}

}

// Make a broadcast file to be waited on.
func (c *Cond) createBcastFile() {
	err := c.MakeFile(c.bcastPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		if c.strict {
			log.Fatalf("Error condvar createBcastFile MakeFile: %v", err)
		} else {
			log.Printf("Error condvar createBcastFile MakeFile: %v", err)
		}
	}
}

// XXX ephemeral?
func (c *Cond) createSignalFile() (string, error) {
	signalFname := rand.String(16)
	signalPath := path.Join(c.path, signalFname)
	err := c.MakeFile(signalPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		db.DLPrintf("CONDVAR", "Error MakeFile Cond.createSignalFile: %v", err)
	}
	return signalPath, err
}
