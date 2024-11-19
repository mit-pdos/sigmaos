package cgroup

import (
	"bufio"
	"os"
	"strconv"

	db "sigmaos/debug"
)

type file struct {
	*os.File
	mode int
}

func newFile(f *os.File, mode int) *file {
	return &file{
		File: f,
		mode: mode,
	}
}

type cgroupFs struct {
	files     map[int]map[string]*file
	sysStatBr *bufio.Reader
}

func newCgroupFs() *cgroupFs {
	// check cgroups V2 are enabled
	checkCgroupsV2()
	return &cgroupFs{
		files: map[int]map[string]*file{
			os.O_RDONLY: make(map[string]*file),
			os.O_RDWR:   make(map[string]*file),
		},
	}
}

// Read the value of a file, according to a parsing function.
func (cfs *cgroupFs) readFile(p string, fn parseFn) (uint64, error) {
	f, err := cfs.getFile(p, os.O_RDONLY)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error get FD: %v", err)
		return 0, err
	}
	return fn(f)
}

func (cfs *cgroupFs) readFileMulti(p string, fn parseFnMulti) ([]int, error) {
	f, err := cfs.getFile(p, os.O_RDONLY)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error get FD: %v", err)
		return nil, err
	}
	return fn(f)
}

// Write the value of a file.
func (cfs *cgroupFs) writeFile(p string, val uint64) error {
	f, err := cfs.getFile(p, os.O_RDWR)
	if err != nil {
		db.DPrintf(db.CGROUP_ERR, "Error get FD: %v", err)
		return err
	}
	defer cfs.closeFile(f)
	b := []byte(strconv.FormatUint(val, 10))
	n2, err := f.Write(b)
	if err != nil || n2 != len(b) {
		db.DPrintf(db.CGROUP_ERR, "Error write: %v n: %v", err, n2)
		return err
	}
	return nil
}

// Return a file descriptor for a file, or open the file if the file descriptor
// doesn't exist.
func (cfs *cgroupFs) getFile(p string, mode int) (*file, error) {
	var f *file
	var ok bool
	if f, ok = cfs.files[mode][p]; !ok {
		var fd *os.File
		var err error
		if fd, err = os.OpenFile(p, mode, 0); err != nil {
			db.DPrintf(db.ALWAYS, "Can't open %v", p)
			return nil, err
		}
		f = newFile(fd, mode)
		cfs.files[mode][p] = f
	} else {
		// Seek to beginning of the file, if it was already open.
		off, err := f.Seek(0, 0)
		if err != nil || off != 0 {
			db.DPrintf(db.ERROR, "Error seeking in file: off %v err %v", off, err)
			return nil, err
		}
	}
	return f, nil
}

func (cfs *cgroupFs) closeFile(f *file) {
	var ok bool
	if _, ok = cfs.files[f.mode][f.Name()]; !ok {
		db.DFatalf("Try to file with no open FD: %v", f.Name())
	}
	delete(cfs.files[f.mode], f.Name())
	if err := f.Close(); err != nil {
		db.DFatalf("Error close fd: path %v err %v", f.Name(), err)
	}
}
