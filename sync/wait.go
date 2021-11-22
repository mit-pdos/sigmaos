package sync

import (
	"log"
	"path"
	"runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/rand"
)

//
// Wait variable specialized for one proc (e.g., parent) waiting on
// another proc (e.g. child).
//
// XXX simplify for one waiter?
//

type Wait struct {
	path string // Path for wait variable
	*fslib.FsLib
}

func MakeWait(fsl *fslib.FsLib, dir string, wait string) *Wait {
	c := &Wait{}
	c.path = path.Join(dir, wait)
	c.FsLib = fsl
	return c
}

// Initialize wait variable by creating its sigmaOS state. This should
// only ever be called once globally.
func (c *Wait) Init() error {
	err := c.Mkdir(c.path, 0777)
	if err != nil {
		db.DLPrintf("WAIT", "MkDir %v err %v", c.path, err)
		return err
	}
	return nil
}

// Wake up a wait waiter; return number of waiters left
func (c *Wait) signal() int {
	wFiles, err := c.ReadDir(c.path)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("Error ReadDir %v in Wait.Signal: %v", c.path, err)
	}

	var waiter string
	for {
		// If no one was waiting, return
		if len(wFiles) == 0 {
			return 0
		}
		i := rand.Int64(int64(len(wFiles)))
		waiter = wFiles[i].Name
		// Make sure we don't remove the broadcast file accidentally
		break
	}

	err = c.Remove(path.Join(c.path, waiter))
	if err != nil {
		log.Fatalf("Error Remove in Wait.Signal: %v", err)
	}
	return len(wFiles) - 1
}

// Wait on wait variable
func (c *Wait) Wait() error {
	signalFname := rand.String(16)
	signalPath := path.Join(c.path, signalFname)
	// XXX ephemeral?
	err := c.MakeFile(signalPath, 0777, np.OWRITE, []byte{})
	if err != nil {
		if strings.HasPrefix(err.Error(), "file not found") {
			return nil
		}
		log.Fatalf("MakeFile %v err %v\n", signalPath, err)
	}
	signal := make(chan error)
	err = c.SetRemoveWatch(signalPath, func(p string, err error) {
		if err != nil {
			log.Printf("func %v err %v\n", signalPath, err)
		}
		signal <- err
	})
	if err == nil {
		err = <-signal
	} else {
		// If error, don't wait.
		if strings.HasPrefix(err.Error(), "file not found") {
			return nil
		}
		log.Fatalf("SetRemoveWatch %v err %v", signalPath, err)
	}
	return err
}

// Signal procs waiting on a wait variable.  XXX race between Signal
// and RmDir: new waiter in between those too
func (c *Wait) Signal() {
	n := c.signal()
	if n > 0 {
		log.Printf("Destroy: more waiters\n")
	}
	// Remove the directory so we don't take on any more waiters
	err := c.RmDir(c.path)
	if err != nil {
		// procclnt wait may already removed dir containing c.path
		if strings.HasPrefix(err.Error(), "file not found") {
			return
		}
		log.Printf("Signal: RmDir %v err %v", c.path, err)
		return
	}
}
