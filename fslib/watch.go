package fslib

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Fwatch func([]*sp.Stat) bool

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir)
func (fsl *FsLib) ReadDirWatch(dir string, watch Fwatch) error {
	for {
		sts, rdr, err := fsl.ReadDir(dir)
		if err != nil {
			return err
		}
		if watch(sts) { // keep watching?
			db.DPrintf(db.FSLIB, "ReadDirWatch watch %v\n", dir)
			if err := fsl.DirWatch(rdr.fd); err != nil {
				rdr.Close()
				if serr.IsErrCode(err, serr.TErrVersion) {
					db.DPrintf(db.FSLIB, "DirWatch: Version mismatch %v", dir)
					continue // try again
				}
				return err
			}
			db.DPrintf(db.FSLIB, "DirWatch %v returned\n", dir)
			// dir has changed; read again
		} else {
			rdr.Close()
			return nil
		}
	}
	return nil
}

// Wait until pn isn't present
func (fsl *FsLib) WaitRemove(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.FSLIB, "WaitRemove: ReadDirWatch dir %v\n", dir)
	err := fsl.ReadDirWatch(dir, func(sts []*sp.Stat) bool {
		db.DPrintf(db.FSLIB, "WaitRemove %v %v %v\n", dir, sp.Names(sts), f)
		for _, st := range sts {
			if st.Name == f {
				return true
			}
		}
		return false
	})
	return err
}

// Wait until pn exists
func (fsl *FsLib) WaitCreate(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.FSLIB, "WaitCreate: ReadDirWatch dir %v\n", dir)
	err := fsl.ReadDirWatch(dir, func(sts []*sp.Stat) bool {
		db.DPrintf(db.FSLIB, "WaitCreate %v %v %v\n", dir, sp.Names(sts), f)
		for _, st := range sts {
			if st.Name == f {
				return false
			}
		}
		return true
	})
	return err
}

// Wait until n entries are in the directory
func (fsl *FsLib) WaitNEntries(pn string, n int) error {
	err := fsl.ReadDirWatch(pn, func(sts []*sp.Stat) bool {
		db.DPrintf(db.FSLIB, "%v # entries %v", len(sts), sp.Names(sts))
		return len(sts) < n
	})
	if err != nil {
		return err
	}
	return nil
}
