package fslib

import (
	"io"
	"log"

	np "ulambda/ninep"
)

func (fl *FsLib) ProcessDir(dir string, f func(*np.Stat) bool) (bool, error) {
	isEmpty := true
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
		isEmpty = false
		for _, st := range dirents {
			if f(st) {
				break
			}
		}
	}
	fl.Close(fd)
	return isEmpty, nil
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
