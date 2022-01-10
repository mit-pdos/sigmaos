package semclnt

import (
	"log"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

//
// Binary semaphore
//

type SemClnt struct {
	path string // Path for semaphore variable
	*fslib.FsLib
}

func MakeSemClnt(fsl *fslib.FsLib, semaphore string) *SemClnt {
	c := &SemClnt{}
	c.path = semaphore
	c.FsLib = fsl
	return c
}

// Initialize semaphore variable by creating its sigmaOS state. This should
// only ever be called once globally.
func (c *SemClnt) Init() error {
	return c.MakeFile(c.path, 0777, np.OWRITE, []byte{})
}

// Down semaphore. If not upped yet (i.e., if file exists), block
func (c *SemClnt) Down() error {
	signal := make(chan error)
	err := c.SetRemoveWatch(c.path, func(p string, err error) {
		if err != nil {
			log.Printf("watch %v err %v\n", c.path, err)
		}
		signal <- err
	})
	if err == nil {
		// log.Printf("semaphore wait %v\n", c.path)
		err = <-signal
	}
	// if err, file has been removed (i.e., semaphore has been
	// "upped" or file server crashed or lease expired).
	// XXX distinguish those cases?
	if err != nil && !strings.HasPrefix(err.Error(), "file not found") {
		log.Printf("%v: down err %v\n", db.GetName(), err)
	}
	return err
}

// Up a semaphore variable (i.e., remove semaphore to indicate up has
// happened).
func (c *SemClnt) Up() error {
	return c.Remove(c.path)
}
