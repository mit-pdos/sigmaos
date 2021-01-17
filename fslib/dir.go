package fslib

import (
	"io"
	"log"

	np "ulambda/ninep"
)

// Too stop early, f must return true.  Returns true if stopped early.
func (fl *FsLib) ProcessDir(dir string, f func(*np.Stat) bool) (bool, error) {
	stopped := false
	fd, err := fl.Opendir(dir)
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
	fd, err := fl.Opendir(dir)
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
