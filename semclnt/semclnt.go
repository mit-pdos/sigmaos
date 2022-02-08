package semclnt

import (
	"log"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
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
func (c *SemClnt) Init(perm np.Tperm) error {
	return c.MakeFile(c.path, 0777|perm, np.OWRITE, []byte{})
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
	// If err, file has been removed (i.e., semaphore has been
	// "upped" or file server crashed or lease expired);
	// distinguish between those cases.
	if err != nil && !np.IsErrNotfound(err) {
		log.Printf("%v: down err %v\n", proc.GetProgram(), err)
		return err
	} else {
		return nil
	}
}

// Up a semaphore variable (i.e., remove semaphore to indicate up has
// happened).
func (c *SemClnt) Up() error {
	return c.Remove(c.path)
}
