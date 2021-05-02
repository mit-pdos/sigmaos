package fslib

import (
	"fmt"

	db "ulambda/debug"
	np "ulambda/ninep"
)

func (fsl *FsLib) PostService(srvaddr, srvname string) error {
	err := fsl.Remove(srvname)
	if err != nil {
		db.DLPrintf("FSCLNT", "Remove failed %v %v\n", srvname, err)
	}
	err = fsl.Symlink(srvaddr+":pubkey", srvname, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) PostServiceUnion(srvaddr, srvname, server string) error {
	p := srvname + "/" + server
	dir, err := fsl.IsDir(srvname)
	if err != nil {
		err := fsl.Mkdir(srvname, 0777)
		if err != nil {
			return err
		}
		dir = true
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	err = fsl.Remove(p)
	if err != nil {
		db.DLPrintf("FSCLNT", "Remove failed %v %v\n", p, err)
	}
	err = fsl.Symlink(srvaddr+":pubkey", p, 0777|np.DMTMP)
	return err
}
