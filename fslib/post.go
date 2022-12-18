package fslib

import (
	"fmt"
	// "log"

	// db "sigmaos/debug"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) MountService(pn string, mnt sp.Tmount) error {
	return fsl.PutFileAtomic(pn, 0777|sp.DMTMP|sp.DMSYMLINK, []byte(mnt.Mnt))
}

// For code running using /mnt/9p, which doesn't support PutFile.
func (fsl *FsLib) MkMountSymlink9P(pn string, mnt sp.Tmount) error {
	return fsl.Symlink([]byte(mnt.Mnt), pn, 0777|sp.DMTMP)
}

func (fsl *FsLib) MountServiceUnion(pn string, mnt sp.Tmount, name string) error {
	p := pn + "/" + name
	dir, err := fsl.IsDir(pn)
	if err != nil {
		return err
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	err = fsl.Symlink([]byte(mnt.Mnt), p, 0777|sp.DMTMP)
	return err
}

func (fsl *FsLib) MkMountSymlink(pn string, mnt sp.Tmount) error {
	if path.EndSlash(pn) {
		return fsl.MountServiceUnion(pn, mnt, sp.Address(mnt))
	} else {
		return fsl.MountService(pn, mnt)
	}
}
