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
// been created in dir). Return whether the ReadDir succeeded or not, so
// that caller can learn if the ReadDir failed or the watch failed.
func (fsl *FsLib) readDirWatch(dir string, watch Fwatch) (bool, error) {
	for {
		sts, rdr, err := fsl.ReadDir(dir)
		if err != nil {
			return false, err
		}
		if watch(sts) { // keep watching?
			db.DPrintf(db.FSLIB, "readDirWatch watch %v\n", dir)
			if err := fsl.DirWatch(rdr.fd); err != nil {
				rdr.Close()
				if serr.IsErrCode(err, serr.TErrVersion) {
					db.DPrintf(db.FSLIB, "DirWatch: Version mismatch %v", dir)
					continue // try again
				}
				return true, err
			}
			db.DPrintf(db.FSLIB, "DirWatch %v returned\n", dir)
			// dir has changed; read again
		} else {
			rdr.Close()
			return true, nil
		}
	}
	return true, nil
}

// Wait until pn isn't present
func (fsl *FsLib) WaitRemove(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.FSLIB, "WaitRemove: readDirWatch dir %v\n", dir)
	_, err := fsl.readDirWatch(dir, func(sts []*sp.Stat) bool {
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
	db.DPrintf(db.FSLIB, "WaitCreate: readDirWatch dir %v\n", dir)
	_, err := fsl.readDirWatch(dir, func(sts []*sp.Stat) bool {
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

// Watch for new entries in a directory. Procs be may
// removing/creating files concurrently from the directory, which may
// create dups; FileWatcher filters those across multiple invocations
// to its methods.  (If caller creates a new FileWatcher for method
// invocation, it can filter duplicates for only that invocation.)

type FileWatcher struct {
	*FsLib
	sync.Mutex
	pn   string
	ents map[string]bool
}

func NewFileWatcher(fslib *FsLib, pn string) *FileWatcher {
	fw := &FileWatcher{
		FsLib: fslib,
		pn:    pn,
		ents:  make(map[string]bool),
	}
	return fw
}

// Wait until n entries are in the directory
func (fw *FileWatcher) WaitNEntries(n int) error {
	_, err := fw.ProcessDir(fw.pn, func(st *sp.Stat) (bool, error) {
		if !fw.ents[st.Name] {
			fw.ents[st.Name] = true
		}
		// stop when we have n entries
		return len(fw.ents) >= n, nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Return unique ents since last call
func (fw *FileWatcher) GetUniqueEntries() ([]string, error) {
	newents := make([]string, 0)
	_, err := fw.ProcessDir(fw.pn, func(st *sp.Stat) (bool, error) {
		if !fw.ents[st.Name] {
			fw.ents[st.Name] = true
			newents = append(newents, st.Name)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return newents, nil
}

// Watch for a directory change relative to present change and return
// all (unique) entries.  Both present and sts are sorted.
func (fw *FileWatcher) WatchUniqueEntries(present []string) ([]string, bool, error) {
	newents := make([]string, 0)
	ok, err := fw.readDirWatch(fw.pn, func(sts []*sp.Stat) bool {
		unchanged := true
		for i, st := range sts {
			if !fw.ents[st.Name] {
				fw.ents[st.Name] = true
				newents = append(newents, st.Name)
				if i >= len(present) || present[i] != st.Name {
					// st.Name is not present return out of readDirWatch
					unchanged = false
				}
			}
		}
		return unchanged
	})
	if err != nil {
		return nil, ok, err
	}
	return newents, ok, nil
}

// Watch for a directory change and return only if new ents
// are present
func (fw *FileWatcher) WatchNewUniqueEntries() ([]string, error) {
	newents := make([]string, 0)
	_, err := fw.readDirWatch(fw.pn, func(sts []*sp.Stat) bool {
		for _, st := range sts {
			if !fw.ents[st.Name] {
				fw.ents[st.Name] = true
				newents = append(newents, st.Name)
			}
		}
		if len(newents) > 0 {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return newents, nil
}

// GetEntsRename gets fw.pn's unique entries and renames them without blocking
func (fw *FileWatcher) GetEntriesRename(dst string) ([]string, error) {
	sts, err := fw.GetDir(fw.pn)
	if err != nil {
		return nil, err
	}
	newents, err := fw.rename(sts, dst)
	if err != nil {
		return nil, err
	}
	return newents, nil
}

// Watch for new entries in fw.pn and return unique renamed entries.
func (fw *FileWatcher) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	var newents []string
	_, err := fw.readDirWatch(fw.pn, func(sts []*sp.Stat) bool {
		db.DPrintf(db.MR, "readDirWatch: %v\n", sts)
		newents, r = fw.rename(sts, dst)
		if r != nil || len(newents) > 0 {
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
	db.DPrintf(db.MR, "readDirWatch: return %v\n", newents)
	return newents, nil
}

// Filter out duplicates and rename
func (fw *FileWatcher) rename(sts []*sp.Stat, dst string) ([]string, error) {
	var r error
	newents := make([]string, 0)
	for _, st := range sts {
		if !fw.ents[st.Name] {
			if err := fw.Rename(filepath.Join(fw.pn, st.Name), filepath.Join(dst, st.Name)); err == nil {
				newents = append(newents, st.Name)
			} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
				r = err
				break
			}
			// another proc renamed the file before us
			fw.ents[st.Name] = true // filter duplicates
		}
	}
	return newents, r
}
