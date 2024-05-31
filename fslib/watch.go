package fslib

import (
	"path/filepath"
	"sync"

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

// Watch for new files in a directory. Procs be may removing/creating
// files concurrently from the directory, which may create dups;
// FileWatcher filters those across multiple invocations to its
// methods.  (If caller creates a new FileWatcher for method
// invocation, it can filter duplicates for only that invocation.)

type FileWatcher struct {
	*FsLib
	sync.Mutex
	pn    string
	files map[string]bool
}

func NewFileWatcher(fslib *FsLib, pn string) *FileWatcher {
	fw := &FileWatcher{
		FsLib: fslib,
		pn:    pn,
		files: make(map[string]bool),
	}
	return fw
}

// Watch for unique files since last call
func (fw *FileWatcher) WatchNewUniqueFiles() ([]string, error) {
	newfiles := make([]string, 0)
	err := fw.ReadDirWatch(fw.pn, func(sts []*sp.Stat) bool {
		for _, st := range sts {
			if !fw.files[st.Name] {
				newfiles = append(newfiles, st.Name)
			}
		}
		if len(newfiles) > 0 {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return newfiles, nil
}

// GetFilesRename gets fw.pn's unique entries and renames them without blocking
func (fw *FileWatcher) GetFilesRename(dst string) ([]string, error) {
	sts, err := fw.GetDir(fw.pn)
	if err != nil {
		return nil, err
	}
	newfiles, err := fw.rename(sts, dst)
	if err != nil {
		return nil, err
	}
	return newfiles, nil
}

// Watch for new entries in fw.pn if none are present. It returns
// unique renamed entries.
func (fw *FileWatcher) WatchNewFilesAndRename(dst string) ([]string, error) {
	var r error
	var newfiles []string
	err := fw.ReadDirWatch(fw.pn, func(sts []*sp.Stat) bool {
		db.DPrintf(db.MR, "ReadDirWatch: %v\n", sts)
		newfiles, r = fw.rename(sts, dst)
		if r != nil || len(newfiles) > 0 {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if r != nil {
		return nil, r
	}
	db.DPrintf(db.MR, "ReadDirWatch: return %v\n", newfiles)
	return newfiles, nil
}

// Filter out duplicates and rename
func (fw *FileWatcher) rename(sts []*sp.Stat, dst string) ([]string, error) {
	var r error
	newfiles := make([]string, 0)
	for _, st := range sts {
		if !fw.files[st.Name] {
			if err := fw.Rename(filepath.Join(fw.pn, st.Name), filepath.Join(dst, st.Name)); err == nil {
				newfiles = append(newfiles, st.Name)
			} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
				r = err
				break
			}
			// another proc renamed the file before us

			fw.files[st.Name] = true // filter duplicates
		}
	}
	return newfiles, r
}
