package fslib

import (
	"errors"
	"fmt"
	"io"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/reader"
)

func (fl *FsLib) MkDir(path string, perm np.Tperm) error {
	perm = perm | np.DMDIR
	fd, err := fl.Create(path, perm, np.OREAD)
	if err != nil {
		return err
	}
	fl.Close(fd)
	return nil
}

func (fl *FsLib) IsDir(name string) (bool, error) {
	st, err := fl.Stat(name)
	if err != nil {
		return false, err
	}
	return st.Mode.IsDir(), nil
}

// Too stop early, f must return true.  Returns true if stopped early.
func (fl *FsLib) ProcessDir(dir string, f func(*np.Stat) (bool, error)) (bool, error) {
	rdr, err := fl.OpenReader(dir)
	if err != nil {
		return false, err
	}
	defer rdr.Close()
	drdr := rdr.NewDirReader()
	for {
		st, err := npcodec.UnmarshalDirEnt(drdr)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, err
		}
		stop, error := f(st)
		if stop {
			return true, error
		}
	}
	return false, nil
}

func (fl *FsLib) GetDir(dir string) ([]*np.Stat, error) {
	st, rdr, err := fl.ReadDir(dir)
	if rdr != nil {
		rdr.Close()
	}
	return st, err
}

func (fl *FsLib) ReadDir(dir string) ([]*np.Stat, *reader.Reader, error) {
	rdr, err := fl.OpenReader(dir)
	if err != nil {
		return nil, nil, err
	}
	dirents := []*np.Stat{}
	drdr := rdr.NewDirReader()
	for {
		st, err := npcodec.UnmarshalDirEnt(drdr)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, rdr, err
		}
		dirents = append(dirents, st)
	}
	return dirents, rdr, nil
}

// XXX should use Reader
func (fl *FsLib) CopyDir(src, dst string) error {
	_, err := fl.ProcessDir(src, func(st *np.Stat) (bool, error) {
		s := src + "/" + st.Name
		d := dst + "/" + st.Name
		db.DPrintf("FSLIB", "CopyFile: %v %v\n", s, d)
		b, err := fl.GetFile(s)
		if err != nil {
			return true, err
		}
		_, err = fl.PutFile(d, 0777, np.OWRITE, b)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	return err
}

func (fl *FsLib) MoveFiles(src, dst string) (int, error) {
	sts, err := fl.GetDir(src) // XXX handle one entry at the time?
	if err != nil {
		return 0, err
	}
	n := 0
	for _, st := range sts {
		db.DPrintf(db.ALWAYS, "move %v to %v\n", st.Name, dst)
		to := dst + "/" + st.Name
		if fl.Rename(src+"/"+st.Name, to) != nil {
			return n, err
		}
		n += 1
	}
	return n, nil
}

// Reads directory incrementally
func (fl *FsLib) RmDirLarge(dir string) error {
	db.DPrintf(db.ALWAYS, "RmDL %v", dir)
	fl.ProcessDir(dir, func(st *np.Stat) (bool, error) {
		if st.Mode.IsDir() {
			err := fl.RmDir(dir + "/" + st.Name)
			if err != nil {
				return true, fmt.Errorf("rmdir %v err %v\n", dir+"/"+st.Name, err)
			}
		} else {
			db.DPrintf(db.ALWAYS, "RmDL remove %v", dir+"/"+st.Name)
			err := fl.Remove(dir + "/" + st.Name)
			db.DPrintf(db.ALWAYS, "RmDL remove %v res %v", dir+"/"+st.Name, err)
			if err != nil {
				return true, err
			}
		}
		return false, nil
	})
	return fl.Remove(dir)
}

func (fsl *FsLib) RmDir(dir string) error {
	if err := fsl.RmDirEntries(dir); err != nil {
		return err
	}
	return fsl.Remove(dir)
}

func (fsl *FsLib) RmDirEntries(dir string) error {
	sts, err := fsl.GetDir(dir)
	if err != nil {
		return err
	}
	for _, st := range sts {
		if st.Mode.IsDir() {
			if err := fsl.RmDir(dir + "/" + st.Name); err != nil {
				return err
			}
		} else {
			if err := fsl.Remove(dir + "/" + st.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fsl *FsLib) SprintfDir(d string) (string, error) {
	return fsl.sprintfDirIndent(d, "")
}

func (fsl *FsLib) sprintfDirIndent(d string, indent string) (string, error) {
	s := fmt.Sprintf("%v dir %v\n", indent, d)
	sts, err := fsl.GetDir(d)
	if err != nil {
		return "", err
	}
	for _, st := range sts {
		s += fmt.Sprintf("%v %v %v\n", indent, st.Name, st.Qid.Type)
		if st.Mode.IsDir() {
			s1, err := fsl.sprintfDirIndent(d+"/"+st.Name, indent+" ")
			if err != nil {
				return s, err
			}
			s += s1
		}
	}
	return s, nil
}

func Present(sts []*np.Stat, names []string) bool {
	n := 0
	m := make(map[string]bool)
	for _, n := range names {
		m[n] = true
	}
	for _, st := range sts {
		if _, ok := m[st.Name]; ok {
			n += 1
		}
	}
	return n == len(names)
}

type Fwait func([]*np.Stat) bool

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir)
func (fsl *FsLib) ReadDirWatch(dir string, wait Fwait) ([]*np.Stat, error) {
	for {
		sts, rdr, err := fsl.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		if wait(sts) { // wait for new inputs?
			ch := make(chan error)
			if err := fsl.SetDirWatch(rdr.Fid(), dir, func(p string, r error) {
				ch <- r
			}); err != nil {
				rdr.Close()
				if np.IsErrVersion(err) {
					db.DPrintf(db.ALWAYS, "ReadDirWatch: Version mismatch %v\n", dir)
					continue
				}
				return nil, err
			}
			if err := <-ch; err != nil {
				rdr.Close()
				return nil, err
			}
			rdr.Close()
			continue // read again
		}
		rdr.Close()
		return sts, nil
	}
	return nil, nil
}
