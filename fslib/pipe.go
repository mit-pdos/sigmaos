package fslib

import (
	np "sigmaos/sigmap"
)

func (fl *FsLib) MakePipe(name string, lperm np.Tperm) error {
	lperm = lperm | np.DMNAMEDPIPE
	// ORDWR so that close doesn't do anything to the pipe state
	fd, err := fl.Create(name, lperm, np.ORDWR)
	if err != nil {
		return err
	}
	return fl.Close(fd)
}
