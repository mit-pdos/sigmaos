package fslib

import (
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
	return npcodec.Byte2Dir(data)
}

// Too stop early, f must return true.  Returns true if stopped early.
func (fl *FsLib) ProcessDir(dir string, f func(*np.Stat) (bool, error)) (bool, error) {
	stopped := false
	var err error
	fd, err := fl.Open(dir, np.OREAD)
	if err != nil {
		return false, err
	}
	for {
		dirents, err := fl.Readdir(fd, CHUNKSZ)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		for _, st := range dirents {
			stopped, err = f(st)
			if stopped {
				break
			}
		}
	}
	fl.Close(fd)
	return stopped, err
}

func (fl *FsLib) ReadDir(dir string) ([]*np.Stat, error) {
	fd, err := fl.Open(dir, np.OREAD)
	if err != nil {
		return nil, err
	}
	dirents := []*np.Stat{}
	for {
		dents, err := fl.Readdir(fd, CHUNKSZ)
		db.DLPrintf("FSLIB", "readdir: %v %v\n", dents, err)
		if err == io.EOF {
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

func (fl *FsLib) CopyDir(src, dst string) error {
	_, err := fl.ProcessDir(src, func(st *np.Stat) (bool, error) {
		s := src + "/" + st.Name
		d := dst + "/" + st.Name
		// db.DLPrintf("FSLIB", "CopyFile: %v %v\n", s, d)
		b, err := fl.ReadFile(s)
		if err != nil {
			return true, err
		}
		err = fl.MakeFile(d, 0777, np.OWRITE, b)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	return err
}

func (fl *FsLib) RmDir(dir string) error {
	fl.ProcessDir(dir, func(st *np.Stat) (bool, error) {
		err := fl.Remove(dir + "/" + st.Name)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	return fl.Remove(dir)
}
