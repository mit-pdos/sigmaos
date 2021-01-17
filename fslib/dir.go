package fslib

import (
	"io"
	"log"

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

func (fl *FsLib) Readdir(fd int, n np.Tsize) ([]*np.Stat, error) {
	data, err := fl.Read(fd, n)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, io.EOF
	}
	dirents := []*np.Stat{}
	for len(data) > 0 {
		st := np.Stat{}
		err = npcodec.Unmarshal(data, &st)
		if err != nil {
			return dirents, err
		}
		dirents = append(dirents, &st)
		sz := np.Tsize(npcodec.SizeNp(st))
		data = data[sz:]
	}
	return dirents, err
}

// Too stop early, f must return true.  Returns true if stopped early.
func (fl *FsLib) ProcessDir(dir string, f func(*np.Stat) bool) (bool, error) {
	stopped := false
	fd, err := fl.Open(dir, np.OREAD)
	if err != nil {
		log.Fatal("Opendir error ", err)
	}
	for {
		dirents, err := fl.Readdir(fd, 1024)
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		for _, st := range dirents {
			stopped = f(st)
			if stopped {
				break
			}
		}
	}
	fl.Close(fd)
	return stopped, nil
}

func (fl *FsLib) ReadDir(dir string) ([]*np.Stat, error) {
	fd, err := fl.Open(dir, np.OREAD)
	if err != nil {
		return nil, err
	}
	dirents := []*np.Stat{}
	for {
		dents, err := fl.Readdir(fd, 1024)
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
