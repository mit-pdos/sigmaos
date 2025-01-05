package dirreader

import (
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt/fslib"
	"strconv"
)

type DirReader interface {
	// Gets the path of the directory being watched
	GetPath() string

	// Gets the (potentially stale) list of files in the directory
	GetDir() ([]string, error)
	Close() error

	// Waits for a file to be removed from the directory
	WaitRemove(file string) error

	// Waits for a file to be created in the directory
	WaitCreate(file string) error

	// Waits for n entries to be in the directory
	// for V1, this does not account for deletions
	// for V2, this accounts for deletions
	WaitNEntries(n int) error

	// Waits for the directory to be empty
	WaitEmpty() error

	// Watch for any directory additions not in present and then return
	// all added entries. This could include entries in present if they were not
	// already in the directory. If provided, any file beginning with an
	// excluded prefix is ignored. present should be sorted.
  // 
	// Also returns a boolean indicating whether the initial read of the directory
	// was successful or not. This is only applicable to V1 and was kept for compatability
	// purposes. In V2, this is always true
	WatchEntriesChangedRelative(present []string, excludedPrefixes []string) ([]string, bool, error)

	// Watch for a directory change and then return all directory entry changes since the last call to
	// a Watch method. For V1, this can have unintended behavior if combined with other Watch methods due to
	// some methods using the cache differently. For V2, this is not an issue
	WatchEntriesChanged() (map[string]bool, error)

// Uses rename to move all entries in the directory to dst. If there are no further entries to be renamed,
// waits for a new entry and then moves it.
	WatchNewEntriesAndRename(dst string) ([]string, error)

	// Uses rename to move all entries in the directory to dst. Can be potentially stale in V2, so combining it
	// with other Watch calls may be desired to get the cache up to date.
	// Does not block if there are no entries to rename
	GetEntriesAndRename(dst string) ([]string, error)
}

type DirReaderVersion int

const (
	V1 DirReaderVersion = 1
	V2 DirReaderVersion = 2
)

func GetDirReaderVersion(pe *proc.ProcEnv) DirReaderVersion {
	if pe.DirReaderVersion == "" {
		return V2
	} else if pe.DirReaderVersion == strconv.Itoa(int(V1)) {
		return V1
	} else if pe.DirReaderVersion == strconv.Itoa(int(V2)) {
		return V2
	} else {
		db.DFatalf("Unknown DirReaderVersion %v\n", pe.DirReaderVersion)
		return V2
	}
}

func NewDirReader(fslib *fslib.FsLib, pn string) (DirReader, error) {
	version := GetDirReaderVersion(fslib.ProcEnv())
	db.DPrintf(db.WATCH, "NewDirReader: version %v\n", version)
	if version == V1 {
		return NewDirReaderV1(fslib, pn), nil
	} else if version == V2 {
		return NewDirReaderV2(fslib, pn)
	} else {
		db.DFatalf("NewDirReader: Unknown DirReaderVersion %v\n", version)
		return nil, nil
	}
}

func WaitRemove(fsl *fslib.FsLib, pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH, "WaitRemove: waiting for %v in dir %v\n", f, dir)
	dirreader, err := NewDirReader(fsl, dir)
	if err != nil {
		return err
	}
	err = dirreader.WaitRemove(f)
	if err != nil {
		return err
	}
	err = dirreader.Close()
	return err
}

func WaitCreate(fsl *fslib.FsLib, pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH, "WaitCreate: waiting for %v in dir %v\n", f, dir)
	dirreader, err := NewDirReader(fsl, dir)
	if err != nil {
		return err
	}
	err = dirreader.WaitCreate(f)
	if err != nil {
		return err
	}
	dirreader.Close()
	return err
}