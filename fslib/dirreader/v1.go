package dirreader

import (
	"path/filepath"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Fwatch func([]*sp.Stat) bool

// Watch for new entries in a directory. Procs may be
// removing/creating files concurrently from the directory, which may
// create dupes; DirReader filters those across multiple invocations
// to its methods.  (If caller creates a new DirReader for method
// invocation, it can filter duplicates for only that invocation.)

type DirReaderV1 struct {
	*fslib.FsLib
	sync.Mutex
	pn   string
	ents map[string]bool
}

func NewDirReaderV1(fslib *fslib.FsLib, pn string) *DirReaderV1 {
	fw := &DirReaderV1{
		FsLib: fslib,
		pn:    pn,
		ents:  make(map[string]bool),
	}
	return fw
}

func (dr *DirReaderV1) Close() error {
	return nil
}

func (dr *DirReaderV1) GetPath() string {
	return dr.pn
}

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir). Return whether the ReadDir succeeded or not, so
// that caller can learn if the ReadDir failed or the watch failed.
func (dr *DirReaderV1) readDirWatch(watch Fwatch) (bool, error) {
	for {
		sts, rdr, err := dr.ReadDir(dr.pn)
		if err != nil {
			return false, err
		}
		if watch(sts) { // keep watching?
			db.DPrintf(db.WATCH, "readDirWatch watch %v\n", dr.pn)
			if err := dr.DirWatch(rdr.Fd()); err != nil {
				rdr.Close()
				if serr.IsErrCode(err, serr.TErrVersion) {
					db.DPrintf(db.WATCH, "DirWatch: Version mismatch %v", dr.pn)
					continue // try again
				}
				return true, err
			}
			db.DPrintf(db.WATCH, "DirWatch %v returned\n", dr.pn)
			// dir has changed; read again
		} else {
			rdr.Close()
			return true, nil
		}
	}
}

func (dr *DirReaderV1) WaitRemove(file string) error {
	db.DPrintf(db.WATCH, "WaitRemove: file %v\n", file)
	_, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		db.DPrintf(db.WATCH, "WaitRemove %v %v\n", sp.Names(sts), file)
		for _, st := range sts {
			if st.Name == file {
				return true
			}
		}
		return false
	})
	return err
}

func (dr *DirReaderV1) WaitCreate(file string) error {
	db.DPrintf(db.WATCH, "WaitCreate: file %v\n", file)
	_, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		db.DPrintf(db.WATCH, "WaitCreate %v %v\n", sp.Names(sts), file)
		for _, st := range sts {
			if st.Name == file {
				return false
			}
		}
		return true
	})
	return err
}

func (dr *DirReaderV1) GetDir() ([]string, error) {
	sts, err := dr.FsLib.GetDir(dr.pn)
	if err != nil {
		return nil, err
	}
	ents := make([]string, 0)
	for _, st := range sts {
		ents = append(ents, st.Name)
	}
	return ents, nil
}

func (dr *DirReaderV1) WaitNEntries(n int) error {
	db.DPrintf(db.WATCH, "WaitNEntries: pn=%s n=%v\n", dr.pn, n)
	_, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		for _, st := range sts {
			if !dr.ents[st.Name] {
				dr.ents[st.Name] = true
			}
		}
		// stop when we have n entries
		return len(dr.ents) < n
	})
	if err != nil {
		return err
	}
	return nil
}

func (dr *DirReaderV1) WaitEmpty() error {
	_, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		return len(sts) > 0
	})
	if err != nil {
		return err
	}
	return nil
}

func (dr *DirReaderV1) WatchEntriesChangedRelative(present []string, prefixFilters []string) ([]string, bool, error) {
	db.DPrintf(db.WATCH, "WatchEntriesChangedRelative: %v %v\n", present, prefixFilters)
	newents := make([]string, 0)
	ok, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		unchanged := true
		for i, st := range sts {
			if len(prefixFilters) > 0 {
				skip := false
				for _, pf := range prefixFilters {
					if strings.HasPrefix(st.Name, pf) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
			}
			if !dr.ents[st.Name] {
				dr.ents[st.Name] = true
				newents = append(newents, st.Name)
				if i >= len(present) || present[i] != st.Name {
					// st.Name is not present return out of readDirWatch
					unchanged = false
				}
			}
		}
		db.DPrintf(db.WATCH, "WatchEntriesChangedRelative: %v %v\n", newents, unchanged)
		return unchanged
	})
	if err != nil {
		return nil, ok, err
	}
	return newents, ok, nil
}

func (dr *DirReaderV1) WatchEntriesChanged() (map[string]bool, error) {
	ents := make(map[string]bool)
	_, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		unchanged := true
		stsMap := make(map[string]bool)
		for _, st := range sts {
			if !dr.ents[st.Name] {
				dr.ents[st.Name] = true
				ents[st.Name] = true
				unchanged = false
			}
			stsMap[st.Name] = true
		}
		for file, exists := range dr.ents {
			if !stsMap[file] && exists {
				dr.ents[file] = false
				ents[file] = false
				unchanged = false
			}
		}
		return unchanged
	})

	if err != nil {
		return nil, err
	}
	return ents, nil
}

func (dr *DirReaderV1) GetEntriesAndRename(dst string) ([]string, error) {
	sts, err := dr.FsLib.GetDir(dr.pn)
	if err != nil {
		return nil, err
	}
	newents, err := dr.rename(sts, dst)
	if err != nil {
		return nil, err
	}
	return newents, nil
}

func (dr *DirReaderV1) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	var newents []string
	_, err := dr.readDirWatch(func(sts []*sp.Stat) bool {
		db.DPrintf(db.MR, "readDirWatch: %v\n", sts)
		newents, r = dr.rename(sts, dst)
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
func (dr *DirReaderV1) rename(sts []*sp.Stat, dst string) ([]string, error) {
	var r error
	newents := make([]string, 0)
	for _, st := range sts {
		if !dr.ents[st.Name] {
			if err := dr.Rename(filepath.Join(dr.pn, st.Name), filepath.Join(dst, st.Name)); err == nil {
				newents = append(newents, st.Name)
			} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
				r = err
				break
			}
			// another proc renamed the file before us
			dr.ents[st.Name] = true // filter duplicates
		}
	}
	return newents, r
}
