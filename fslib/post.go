package fslib

import (
	"fmt"
	"log"

	db "ulambda/debug"
	np "ulambda/ninep"
)

func (fsl *FsLib) PostService(srvaddr, srvname string) error {
	err := fsl.Remove(srvname)
	if err != nil {
		log.Printf("%v: Remove failed %v %v\n", db.GetName(), srvname, err)
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
		log.Printf("%v: Remove failed %v %v\n", db.GetName(), p, err)
	}
	err = fsl.Symlink(srvaddr+":pubkey", p, 0777|np.DMTMP)
	return err
}
