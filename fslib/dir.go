package fslib

import (
	"fmt"
	"io"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

func (fl *FsLib) Mkdir(path string, perm np.Tperm) error {
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

func (fl *FsLib) Readdir(fd int, n np.Tsize) ([]*np.Stat, error) {
	data, err := fl.Read(fd, n)
	db.DLPrintf("FSLIB", "Readdir: read -> %d %v\n", len(data), err)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, io.EOF
	}
	if dents, err := npcodec.Byte2Dir(data); err != nil {
		return nil, err
	} else {
		return dents, nil
	}
}

// Too stop early, f must return true.  Returns true if stopped early.
func (fl *FsLib) ProcessDir(dir string, f func(*np.Stat) (bool, error)) (bool, error) {
	fd, err := fl.Open(dir, np.OREAD)
	if err != nil {
		return false, err
	}
	defer fl.Close(fd)
	for {
		dirents, err := fl.Readdir(fd, fl.chunkSz)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		for _, st := range dirents {
			stop, err := f(st)
			if stop {
				return true, err
			}
		}
	}
	return false, err
}

func (fl *FsLib) ReadDir(dir string) ([]*np.Stat, error) {
	fd, err := fl.Open(dir, np.OREAD)
	if err != nil {
		return nil, err
	}
	dirents := []*np.Stat{}
	for {
		dents, err := fl.Readdir(fd, fl.chunkSz)
		db.DLPrintf("FSLIB", "readdir: %v %T%v\n", dents, err, err)
		if err != nil && np.IsErrEOF(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		dirents = append(dirents, dents...)
	}
	fl.Close(fd)
	return dirents, nil
}

// XXX should use Reader
func (fl *FsLib) CopyDir(src, dst string) error {
	_, err := fl.ProcessDir(src, func(st *np.Stat) (bool, error) {
		s := src + "/" + st.Name
		d := dst + "/" + st.Name
		// db.DLPrintf("FSLIB", "CopyFile: %v %v\n", s, d)
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
	sts, err := fsl.ReadDir(dir)
	if err != nil {
		return err
	}
	// log.Printf("%v: rmdir1 %v\n", db.GetName(), dir)
	for _, st := range sts {
		if st.Mode.IsDir() {
			fsl.RmDir(dir + "/" + st.Name)
		} else {
			fsl.Remove(dir + "/" + st.Name)
		}
	}
	return fsl.Remove(dir)
}

func (fsl *FsLib) SprintfDir(d string) (string, error) {
	return fsl.sprintfDirIndent(d, "")
}

func (fsl *FsLib) sprintfDirIndent(d string, indent string) (string, error) {
	s := fmt.Sprintf("%v dir %v\n", indent, d)
	sts, err := fsl.ReadDir(d)
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
