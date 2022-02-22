package fslib

import (
	np "ulambda/ninep"
)

func (fl *FsLib) Symlink(target []byte, link string, lperm np.Tperm) error {
	lperm = lperm | np.DMSYMLINK
	fd, err := fl.Create(link, lperm, np.OWRITE)
	if err != nil {
		return err
	}
	_, err = fl.Write(fd, target)
	if err != nil {
		return err
	}
	return fl.Close(fd)
}
