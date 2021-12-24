package fslib

import (
	"strings"

	np "ulambda/ninep"
)

func (fl *FsLib) SymlinkReplica(targets []string, link string, lperm np.Tperm) error {
	lperm = lperm | np.DMSYMLINK
	fd, err := fl.Create(link, lperm, np.OWRITE)
	if err != nil {
		return err
	}
	_, err = fl.Write(fd, []byte(strings.Join(targets, "\n")))
	if err != nil {
		return err
	}
	return fl.Close(fd)
}

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
