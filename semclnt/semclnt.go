package semclnt

import (
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

//
// Library for binary semaphore, implemented using a file and a watch.
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
	db.DLPrintf("SEMCLNT", "Semaphore init %v\n", c.path)
	_, err := c.PutFile(c.path, 0777|perm, np.OWRITE, []byte{})
	return err
}

// Down semaphore. If not upped yet (i.e., if file exists), block
func (c *SemClnt) Down() error {
	db.DLPrintf("SEMCLNT", "Down %v\n", c.path)
	signal := make(chan error)
	for {
		err := c.SetRemoveWatch(c.path, func(p string, err1 error) {
			if err1 != nil {
				db.DLPrintf("SEMCLNT", "watch %v err %v\n", c.path, err1)
			}
			signal <- err1
		})
		// If err is because file has been removed, then no error: the
		// semaphore has been "upped".
		if err != nil && np.IsErrNotfound(err) {
			db.DLPrintf("SEMCLNT", "down %v ok err %v\n", c.path, err)
			break
		}
		if err == nil {
			db.DLPrintf("SEMCLNT", "semaphore wait %v\n", c.path)
			err = <-signal
		} else {
			db.DLPrintf("SEMCLNT_ERR", "down %v err %v\n", c.path, err)
			return err
		}
		if err != nil && np.IsErrVersion(err) {
			db.DLPrintf("SEMCLNT_ERR", "down %v retry err %v\n", c.path, err)
			continue
		}
		if err != nil {
			db.DLPrintf("SEMCLNT_ERR", "down %v watch err %v\n", c.path, err)
		}
		break
	}
	return nil
}

// Up a semaphore variable (i.e., remove semaphore to indicate up has
// happened).
func (c *SemClnt) Up() error {
	db.DLPrintf("SEMCLNT", "Down %v\n", c.path)
	return c.Remove(c.path)
}
