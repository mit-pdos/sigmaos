package sync

import (
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

//
// Binary semaphore
//

type Semaphore struct {
	path string // Path for semaphore variable
	*fslib.FsLib
}

func MakeSemaphore(fsl *fslib.FsLib, semaphore string) *Semaphore {
	c := &Semaphore{}
	c.path = semaphore
	c.FsLib = fsl
	return c
}

// Initialize semaphore variable by creating its sigmaOS state. This should
// only ever be called once globally.
func (c *Semaphore) Init() error {
	err := c.MakeFile(c.path, 0777, np.OWRITE, []byte{})
	if err != nil {
		db.DLPrintf("SEMAPHORE", "MakeFile %v err %v", c.path, err)
		return err
	}
	return nil
}

// Down semaphore. If file exists (i.e., semaphore = 0), wait.
func (c *Semaphore) Down() error {
	signal := make(chan error)
	err := c.SetRemoveWatch(c.path, func(p string, err error) {
		if err != nil {
			log.Printf("func %v err %v\n", c.path, err)
		}
		signal <- err
	})
	if err == nil {
		log.Printf("semaphore wait %v\n", c.path)
		err = <-signal
	}
	// file has been removed (i.e., semaphore has been "incremented")
	return err
}

// Up a semaphore variable. Remove semaphore to indicate up has happened.
func (c *Semaphore) Up() {
	err := c.Remove(c.path)
	if err != nil {
		log.Fatalf("Remove %v err %v\n", c.path, err)
	}
}
