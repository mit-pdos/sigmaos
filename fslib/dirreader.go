package fslib

import (
	"path/filepath"
	"strings"
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
		db.DPrintf(db.WATCH, "readDirWatch start %v\n", dir)
		sts, rdr, err := fsl.ReadDir(dir)
		db.DPrintf(db.WATCH, "readDirWatch read dir %v\n", dir)
		if err != nil {
			return false, err
		}
		if watch(sts) { // keep watching?
			db.DPrintf(db.WATCH, "readDirWatch watch %v\n", dir)
			if err := fsl.DirWatch(rdr.fd); err != nil {
				rdr.Close()
				if serr.IsErrCode(err, serr.TErrVersion) {
					db.DPrintf(db.WATCH, "DirWatch: Version mismatch %v", dir)
					continue // try again
				}
				return true, err
			}
			db.DPrintf(db.WATCH, "DirWatch %v returned\n", dir)
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
	db.DPrintf(db.WATCH, "WaitRemove: readDirWatch dir %v\n", dir)
	_, err := fsl.readDirWatch(dir, func(sts []*sp.Stat) bool {
		db.DPrintf(db.WATCH, "WaitRemove %v %v %v\n", dir, sp.Names(sts), f)
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
	db.DPrintf(db.WATCH, "WaitCreate: readDirWatch dir %v\n", dir)
	_, err := fsl.readDirWatch(dir, func(sts []*sp.Stat) bool {
		db.DPrintf(db.WATCH, "WaitCreate %v %v %v\n", dir, sp.Names(sts), f)
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
// create dups; DirReader filters those across multiple invocations
// to its methods.  (If caller creates a new DirReader for method
// invocation, it can filter duplicates for only that invocation.)

type DirReader struct {
	*FsLib
	sync.Mutex
	pn   string
	ents map[string]bool
}

func NewDirReader(fslib *FsLib, pn string) *DirReader {
	fw := &DirReader{
		FsLib: fslib,
		pn:    pn,
		ents:  make(map[string]bool),
	}
	return fw
}

// Wait until n entries are in the directory
func (dr *DirReader) WaitNEntries(n int) error {
	_, err := dr.readDirWatch(dr.pn, func(sts []*sp.Stat) bool {
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

func (dr *DirReader) GetUniqueEntries() ([]string, error) {
	return dr.GetUniqueEntriesFilter("")
}

// Return unique ents since last call, filtering names that start with
// prefixFilter.
func (dr *DirReader) GetUniqueEntriesFilter(prefixFilter string) ([]string, error) {
	db.DPrintf(db.WATCH, "GetUniqueEntries %v\n", dr.pn)
	newents := make([]string, 0)
	_, err := dr.ProcessDir(dr.pn, func(st *sp.Stat) (bool, error) {
		db.DPrintf(db.WATCH, "GetUniqueEntries: process entry %v\n", st.Name)
		if prefixFilter != "" && strings.HasPrefix(st.Name, prefixFilter) {
			return false, nil
		}
		if !dr.ents[st.Name] {
			dr.ents[st.Name] = true
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
func (dr *DirReader) WatchUniqueEntries(present []string, prefixFilters []string) ([]string, bool, error) {
	newents := make([]string, 0)
	ok, err := dr.readDirWatch(dr.pn, func(sts []*sp.Stat) bool {
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
		return unchanged
	})
	if err != nil {
		return nil, ok, err
	}
	return newents, ok, nil
}

// Watch for a directory change and return only if new ents
// are present
func (dr *DirReader) WatchNewUniqueEntries() ([]string, error) {
	newents := make([]string, 0)
	_, err := dr.readDirWatch(dr.pn, func(sts []*sp.Stat) bool {
		for _, st := range sts {
			if !dr.ents[st.Name] {
				dr.ents[st.Name] = true
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

// GetEntsRename gets dr.pn's unique entries and renames them without blocking
func (dr *DirReader) GetEntriesRename(dst string) ([]string, error) {
	sts, err := dr.GetDir(dr.pn)
	if err != nil {
		return nil, err
	}
	newents, err := dr.rename(sts, dst)
	if err != nil {
		return nil, err
	}
	return newents, nil
}

// Watch for new entries in dr.pn and return unique renamed entries.
func (dr *DirReader) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	var newents []string
	_, err := dr.readDirWatch(dr.pn, func(sts []*sp.Stat) bool {
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
func (dr *DirReader) rename(sts []*sp.Stat, dst string) ([]string, error) {
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
