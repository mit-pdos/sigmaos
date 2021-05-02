package fslibsrv

import (
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
)

type FsLibSrv struct {
	*fslib.FsLib
	*memfsd.Fsd
}

func (fsl *FsLibSrv) Clnt() *fslib.FsLib {
	return fsl.FsLib
}

func InitFsFsl(name string, fsc *fslib.FsLib, memfsd *memfsd.Fsd, dev memfs.Dev) (*FsLibSrv, error) {
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
	fsl := fslib.MakeFsLib(name)
	return InitFsFsl(name, fsl, memfsd, dev)
}

func (fsl *FsLibSrv) ExitFs(name string) {
	err := fsl.Remove(name)
	if err != nil {
		db.DLPrintf("FSCLNT", "Remove failed %v %v\n", name, err)
	}
}
