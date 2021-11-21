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

type Wait struct {
	path      string // Path to condition variable
	bcastPath string // Path to broadcast file watched by everyone
	strict    bool   // If true, kill on error. Otherwise, print to debug
	*fslib.FsLib
}

func MakeWait(fsl *fslib.FsLib, dir string, cond string) *Wait {
	c := &Wait{}
	c.path = path.Join(dir, cond)
	c.bcastPath = path.Join(c.path, BROADCAST)
	c.FsLib = fsl
	return c
}

// Initialize Wait by creating its sigmaOS state. This should only
// ever be called once globally.
func (c *Wait) Init() error {
	err := c.Mkdir(c.path, 0777)
	if err != nil {
		db.DLPrintf("WAIT", "MkDir %v err %v", c.path, err)
		return err
	}
	c.createBcastFile()
	return nil
}

// Wake up all waiters.
func (c *Wait) Broadcast() {
	err := c.Remove(c.bcastPath)
	if err != nil {
		if c.strict {
			log.Fatalf("Error Remove in Wait.Broadcast: %v", err)
		} else {
			log.Printf("Error Remove in Wait.Broadcast: %v", err)
		}
	}
	c.createBcastFile()
}

// Wake up one waiter.
func (c *Wait) Signal() {
	wFiles, err := c.ReadDir(c.path)
	if err != nil {
		debug.PrintStack()
		if c.strict {
			log.Printf("Error ReadDir %v in Wait.Signal: %v", c.path, err)
		} else {
			log.Fatalf("Error ReadDir %v in Wait.Signal: %v", c.path, err)
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
			log.Printf("Error Remove in Wait.Signal: %v", err)
		} else {
			log.Fatalf("Error Remove in Wait.Signal: %v", err)
		}
	}
}

// Wait.
func (c *Wait) Wait() error {
	done := make(chan error, 2)

	signalPath, err := c.createSignalFile()
	if err != nil {
		return err
	}

	// Everyone waits on the broadcast file
	go func() {
		bcast := make(chan error)
		err := c.SetRemoveWatch(c.bcastPath, func(p string, err error) {
			if err != nil {
				db.DLPrintf("WAIT", "Error RemoveWatch bcast triggered in Wait.Wait: %v", err)
			}
			bcast <- err
		})
		// If error, don't wait.
		if err == nil {
			err = <-bcast
		} else {
			db.DLPrintf("WAIT", "Error SetRemoveWatch bcast Wait.Wait: %v", err)
		}

		c.Remove(signalPath)
		done <- err
	}()

	// Each waiter waits on its own signal file
	go func() {
		signal := make(chan error)
		err := c.SetRemoveWatch(signalPath, func(p string, err error) {
			if err != nil {
				db.DLPrintf("WAIT", "Error RemoveWatch bcast triggered in Wait.Wait: %v", err)
			}
			signal <- err
		})
		// If error, don't wait.
		if err == nil {
			err = <-signal
		} else {
			db.DLPrintf("WAIT", "Error SetRemoveWatch bcast Wait.Wait: %v", err)
		}
		done <- err
	}()

	// Wait for either the Signal or Broadcast watch to be triggered
	err = <-done
	return err
}

// Tear down a Wait variable by waking all waiters.
func (c *Wait) Destroy() {
	// Wake up all waiters with a broadcast.
	err := c.Remove(c.bcastPath)
	if err != nil {
		debug.PrintStack()
		if err.Error() == "EOF" {
			log.Printf("Error Remove 1 in Wait.Destroy: %v", err)
		} else {
			if c.strict {
				log.Fatalf("Error Remove strict 1 in Wait.Destroy: %v", err)
			} else {
				log.Printf("Error Remove 1 in Wait.Destroy: %v", err)
			}
		}
	}
}

// Make a broadcast file to be waited on.
func (c *Wait) createBcastFile() {
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
func (c *Wait) createSignalFile() (string, error) {
	signalFname := rand.String(16)
	signalPath := path.Join(c.path, signalFname)
	err := c.MakeFile(signalPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		db.DLPrintf("WAIT", "Error MakeFile Wait.createSignalFile: %v", err)
	}
	return signalPath, err
}
