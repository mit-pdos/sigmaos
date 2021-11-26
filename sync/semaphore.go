package sync

import (
	"log"

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
	return c.MakeFile(c.path, 0777, np.OWRITE, []byte{})
}

// Down semaphore. If not upped yet (i.e., if file exists), block
func (c *Semaphore) Down() error {
	signal := make(chan error)
	err := c.SetRemoveWatch(c.path, func(p string, err error) {
		if err != nil {
			log.Printf("func %v err %v\n", c.path, err)
		}
		signal <- err
	})
	if err == nil {
		// log.Printf("semaphore wait %v\n", c.path)
		err = <-signal
	}
	// if err, file has been removed (i.e., semaphore has been
	// "upped" or file server crashed).
	// XXX distinguish those cases?
	return nil
}

// Up a semaphore variable (i.e., remove semaphore to indicate up has
// happened).
func (c *Semaphore) Up() error {
	return c.Remove(c.path)
}
