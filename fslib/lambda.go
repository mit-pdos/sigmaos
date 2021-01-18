package fslib

import (
// "log"
// "errors"
//np "ulambda/ninep"
)

func (fl *FsLib) Started(pid string) error {
	return fl.WriteFile("name/ulambd/ulambd", []byte("Started "+pid))
}

func (fl *FsLib) Exit(pid string) error {
	return fl.WriteFile("name/ulambd/ulambd", []byte("Exit "+pid))
}
