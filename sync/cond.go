package sync

import (
	"log"
	"math/rand"
	"path"
	"time"

	"github.com/thanhpk/randstr"

	np "ulambda/ninep"
)

const (
	DIR_LOCK  = "DIR_LOCK"   // Protects cond var directory
	BROADCAST = ".BROADCAST" // A file watched by everone for broadcasts
)

type Cond struct {
	pid       string // Caller's PID
	path      string // Path to condition variable
	bcastPath string // Path to broadcast file watched by everyone
	lockDir   string // Path to the lock's parent dir
	lockName  string // Lock's name
}

func MakeCond(pid, condpath, lockDir, lockName string) *Cond {
	c := &Cond{}
	c.path = condpath
	c.bcastPath = path.Join(condpath, BROADCAST)
	c.lockDir = lockDir
	c.lockName = lockName

	// Make a directory in which waiters register themselves
	err := fsl.Mkdir(c.path, 0777)
	if err != nil {
		log.Fatalf("Error creating cond variable dir: %v", err)
	}

	c.createBcastFile()

	// Seed the random number generator (used to pick random waiter to signal)
	rand.Seed(time.Now().Unix())

	return c
}

// Wait.
func (c *Cond) Wait() {
	fsl.LockFile(c.path, DIR_LOCK)

	waitfilePath := c.createWaitfile()

	// Size 2 so we don't get hanging threads
	done := make(chan bool, 2)

	// Everyone waits on the broadcast file
	go func() {
		bcast := make(chan bool)
		err := fsl.SetRemoveWatch(c.bcastPath, func(p string, err error) {
			if err != nil && err.Error() == "EOF" {
				log.Fatalf("Error RemoveWatch bcast triggered in Cond.Wait: %v", err)
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
		fsl.Remove(waitfilePath)
	}()

	// Everyone waits on their waitfile
	go func() {
		signal := make(chan bool)
		err := fsl.SetRemoveWatch(waitfilePath, func(p string, err error) {
			if err != nil && err.Error() == "EOF" {
				log.Fatalf("Error RemoveWatch signal triggered in Cond.Wait: %v", err)
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

	fsl.UnlockFile(c.path, DIR_LOCK)

	// Wait for either the Signal or Broadcast watch to be triggered
	<-done

	// Lock & return
	fsl.LockFile(c.lockDir, c.lockName)
}

// Wake up all waiters.
func (c *Cond) Broadcast() {
	fsl.LockFile(c.path, DIR_LOCK)
	defer fsl.UnlockFile(c.path, DIR_LOCK)

	err := fsl.Remove(c.bcastPath)
	if err != nil {
		log.Fatalf("Error Remove in Cond.Broadcast: %v", err)
	}

	c.createBcastFile()
}

// Wake up one waiter.
func (c *Cond) Signal() {
	fsl.LockFile(c.path, DIR_LOCK)
	defer fsl.UnlockFile(c.path, DIR_LOCK)

	waiters, err := fsl.ReadDir(c.path)
	if err != nil {
		log.Fatalf("Error ReadDir in Cond.Signal: %v", err)
	}

	// Shuffle the list of waiters
	rand.Shuffle(len(waiters), func(i, j int) { waiters[i], waiters[j] = waiters[j], waiters[i] })

	for _, w := range waiters {
		if w.Name != BROADCAST && w.Name != DIR_LOCK {
			// Wake a single waiter
			err := fsl.Remove(path.Join(c.path, w.Name))
			if err != nil {
				log.Fatalf("Error Remove in Cond.Signal: %v", err)
			}
			return
		}
	}
}

// Make a broadcast file to be waited on.
func (c *Cond) createBcastFile() {
	err := fsl.MakeFile(c.bcastPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		log.Fatalf("Error creating cond variable broadcast file: %v", err)
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
	err := fsl.MakeFile(waitfilePath, 0777, np.OWRITE, []byte{})
	if err != nil {
		log.Fatalf("Error MakeFile in Cond.Wait: %v, %v", waitfilePath, err)
	}
	return waitfilePath
}
