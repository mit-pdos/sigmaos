package sync

import (
	"log"
	"math/rand"
	"path"
	"time"

	"github.com/thanhpk/randstr"

	//	db "ulambda/debug"
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
	pid       string // Caller's PID
	path      string // Path to condition variable
	bcastPath string // Path to broadcast file watched by everyone
	*fslib.FsLib
}

func MakeCond(fsl *fslib.FsLib, pid, condpath string, lock *Lock) *Cond {
	c := &Cond{}
	c.condLock = lock
	c.pid = pid
	c.path = condpath
	c.bcastPath = path.Join(condpath, BROADCAST)
	c.FsLib = fsl

	c.dirLock = MakeLock(fsl, c.path, DIR_LOCK)

	// Seed the random number generator (used to pick random waiter to signal)
	rand.Seed(time.Now().Unix())

	return c
}

// Initialize the condition variable by creating its sigmaOS state. This should
// only ever be called once globally per condition variable.
func (c *Cond) Init() {
	// Make a directory in which waiters register themselves
	err := c.Mkdir(c.path, 0777)
	if err != nil {
		log.Fatalf("Error condvar Init MkDir: %v", err)
	}

	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	c.createBcastFile()
}

// Wake up all waiters. The condLock need not be held, and needs to be manually
// unlocked independently of this function call.
func (c *Cond) Broadcast() {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	err := c.Remove(c.bcastPath)
	if err != nil {
		log.Fatalf("Error Remove in Cond.Broadcast: %v", err)
	}

	c.createBcastFile()
}

// Wake up one waiter. The condLock need not be held, and needs to be manually
// unlocked independently of this function call..
func (c *Cond) Signal() {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	waiters, err := c.ReadDir(c.path)
	if err != nil {
		log.Fatalf("Error ReadDir in Cond.Signal: %v", err)
	}

	// Shuffle the list of waiters
	rand.Shuffle(len(waiters), func(i, j int) { waiters[i], waiters[j] = waiters[j], waiters[i] })

	for _, w := range waiters {
		if w.Name != BROADCAST && w.Name != DIR_LOCK {
			// Wake a single waiter
			err := c.Remove(path.Join(c.path, w.Name))
			if err != nil {
				log.Fatalf("Error Remove in Cond.Signal: %v", err)
			}
			return
		}
	}
}

// Wait. If condLock != nil, assumes the condLock is held, and returns with the
// condLock held once again.
func (c *Cond) Wait() {
	c.dirLock.Lock()

	waitfilePath := c.createWaitfile()

	// Size 2 so we don't get hanging threads
	done := make(chan bool, 2)

	// Everyone waits on the broadcast file
	go func() {
		bcast := make(chan bool)
		err := c.SetRemoveWatch(c.bcastPath, func(p string, err error) {
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil {
				log.Printf("Error RemoveWatch bcast triggered in Cond.Wait: %v", err)
			}
			bcast <- true
		})
		// If error, don't wait.
		if err == nil {
			<-bcast
		} else {
			log.Fatalf("Error SetRemoveWatch bcast Cond.Wait: %v", err)
		}
		done <- true

		// Make sure the waitfile was removed
		c.Remove(waitfilePath)
	}()

	// Everyone waits on their waitfile
	go func() {
		signal := make(chan bool)
		err := c.SetRemoveWatch(waitfilePath, func(p string, err error) {
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil {
				log.Printf("Error RemoveWatch signal triggered in Cond.Wait: %v", err)
			}
			signal <- true
		})
		// If error, don't wait.
		if err == nil {
			<-signal
		} else {
			log.Fatalf("Error SetRemoveWatch signal Cond.Wait: %v", err)
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

// Make a broadcast file to be waited on.
func (c *Cond) createBcastFile() {
	err := c.MakeFile(c.bcastPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		log.Fatalf("Error condvar createBcastFile MakeFile: %v", err)
	}
}

// Create a unique file to be waited on. Name is PID + "." + randstr to
// accommodate multiple threads waiting on a condition from within the same
// process.
func (c *Cond) createWaitfile() string {
	r := randstr.Hex(16)
	id := c.pid + "." + r
	waitfilePath := path.Join(c.path, id)

	// XXX Should be ephemeral?
	err := c.MakeFile(waitfilePath, 0777, np.OWRITE, []byte{})
	if err != nil {
		log.Fatalf("Error MakeFile in Cond.Wait: %v, %v", waitfilePath, err)
	}
	return waitfilePath
}
