package fslib

import (
	"errors"
	"fmt"
	"io"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
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
	rdr, err := fl.OpenReader(dir)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	dirents := []*np.Stat{}
	drdr := rdr.NewDirReader()
	for {
		st, err := npcodec.UnmarshalDirEnt(drdr)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		dirents = append(dirents, st)
	}
	return dirents, nil
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

// Reads directory incrementally
func (fl *FsLib) RmDirLarge(dir string) error {
	fl.ProcessDir(dir, func(st *np.Stat) (bool, error) {
		if st.Mode.IsDir() {
			err := fl.RmDir(dir + "/" + st.Name)
			if err != nil {
				return true, fmt.Errorf("rmdir %v err %v\n", dir+"/"+st.Name, err)
			}
		} else {
			err := fl.Remove(dir + "/" + st.Name)
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
