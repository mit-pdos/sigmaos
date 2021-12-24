package fslib

import (
	"fmt"
	// "log"

	// db "ulambda/debug"
	np "ulambda/ninep"
)

func MakeTarget(srvaddr string) []byte {
	return []byte(srvaddr + ":pubkey")
}

func (fsl *FsLib) PostService(srvaddr, srvname string) error {
	err := fsl.Symlink(MakeTarget(srvaddr), srvname, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) PostServiceUnion(srvaddr, srvpath, server string) error {
	p := srvpath + "/" + server
	dir, err := fsl.IsDir(srvpath)
	if err != nil {
		err := fsl.Mkdir(srvpath, 0777)
		if err != nil {
			return err
		}
		dir = true
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	err = fsl.Symlink(MakeTarget(srvaddr), p, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) Post(srvaddr, path string) error {
	if np.EndSlash(path) {
		return fsl.PostServiceUnion(srvaddr, path, srvaddr)
	} else {
		return fsl.PostService(srvaddr, path)
	}
}
