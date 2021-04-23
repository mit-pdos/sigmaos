package fslib

import (
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

type FsLibSrv struct {
	*FsLib
	*memfsd.Fsd
}

func (fsl *FsLibSrv) Clnt() *FsLib {
	return fsl.FsLib
}

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

func InitFsFsl(name string, fsc *FsLib, memfsd *memfsd.Fsd, dev memfs.Dev) (*FsLibSrv, error) {
	fsl := &FsLibSrv{fsc, memfsd}
	if dev != nil {
		err := memfsd.MkNod("dev", dev)
		if err != nil {
			log.Fatal("Create error: dev: ", err)
		}
	}
	err := fsl.PostService(memfsd.Addr(), name)
	if err != nil {
		return nil, fmt.Errorf("PostService %v error: %v", name, err)
	}
	return fsl, nil
}

func InitFs(name string, memfsd *memfsd.Fsd, dev memfs.Dev) (*FsLibSrv, error) {
	fsl := MakeFsLib(name)
	return InitFsFsl(name, fsl, memfsd, dev)
}

func (fsl *FsLib) ExitFs(name string) {
	err := fsl.Remove(name)
	if err != nil {
		db.DLPrintf("FSCLNT", "Remove failed %v %v\n", name, err)
	}
}
