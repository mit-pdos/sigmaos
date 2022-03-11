package semclnt

import (
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
func (c *SemClnt) Init(perm np.Tperm) error {
	db.DLPrintf("SEMCLNT", "Semaphore init %v\n", c.path)
	_, err := c.PutFile(c.path, 0777|perm, np.OWRITE, []byte{})
	return err
}

// Down semaphore. If not upped yet (i.e., if file exists), block
func (c *SemClnt) Down() error {
	signal := make(chan error)
	err := c.SetRemoveWatch(c.path, func(p string, err error) {
		if err != nil {
			db.DLPrintf("SEMCLNT", "watch %v err %v\n", c.path, err)
		}
		signal <- err
	})
	if err == nil {
		db.DLPrintf("SEMCLNT", "semaphore wait %v\n", c.path)
		err = <-signal
	}
	// If err is because file has been removed, then no error: the
	// semaphore has been "upped". A file is removed when it isn't
	// found or SetRemoveWatch erred with version mismatch; the
	// latter case happens when removed has unliked the file,
	// increasing version #, but remove hasn't removed the
	// underlying inode (e.g., because SetRemoveWatch has fid for
	// the unlinked file)
	if err != nil && (np.IsErrNotfound(err) || np.IsErrVersion(err)) {
		db.DLPrintf("SEMCLNT", "down %v ok err %v\n", c.path, err)
		return nil
	}
	if err != nil {
		db.DLPrintf("SEMCLNT", "down %v err %v\n", c.path, err)
		return err
	}
	return nil
}

// Up a semaphore variable (i.e., remove semaphore to indicate up has
// happened).
func (c *SemClnt) Up() error {
	return c.Remove(c.path)
}
