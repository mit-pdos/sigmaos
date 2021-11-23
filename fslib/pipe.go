package fslib

import (
	np "ulambda/ninep"
)

func (fl *FsLib) MakePipe(name string, lperm np.Tperm) error {
	lperm = lperm | np.DMNAMEDPIPE
	fd, err := fl.Create(name, lperm, np.OWRITE)
	if err != nil {
		return err
	}
	return fl.Close(fd)
}
