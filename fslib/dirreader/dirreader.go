package dirreader

import (
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"strconv"
)

// TODO:
// - change the fidwatch to just be a normal fid with a diff fsobj
// - write some doc comments

type DirReader interface {
	GetPath() string
	GetDir() ([]string, error)
	Close() error
	WaitRemove(file string) error
	WaitCreate(file string) error
	WaitNEntries(n int) error
	WaitEmpty() error
	WatchEntriesChangedRelative(present []string, excludedPrefixes []string) ([]string, bool, error)
	WatchEntriesChanged() (map[string]bool, error)
	WatchNewEntriesAndRename(dst string) ([]string, error)
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
	db.DPrintf(db.WATCH_V2, "NewDirReader: version %v\n", version)
	if version == V1 {
		return NewDirReaderV1(fslib, pn), nil
	} else if version == V2 {
		return NewDirReaderV2(fslib, pn)
	} else {
		db.DFatalf("NewDirReader: Unknown DirReaderVersion %v\n", version)
		return nil, nil
	}
}

// Wait until pn isn't present
func WaitRemove(fsl *fslib.FsLib, pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH_V2, "WaitRemove: waiting for %v in dir %v\n", f, dir)
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

// Wait until pn exists
func WaitCreate(fsl *fslib.FsLib, pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH_V2, "WaitCreate: waiting for %v in dir %v\n", f, dir)
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