package fslibsrv

import (
	"fmt"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
)

type FsLibSrv struct {
	*fslib.FsLib
	*memfsd.Fsd
}

func (fsl *FsLibSrv) Clnt() *fslib.FsLib {
	return fsl.FsLib
}

func InitFsFsl(name string, fsc *fslib.FsLib, memfsd *memfsd.Fsd) (*FsLibSrv, error) {
	fsl := &FsLibSrv{fsc, memfsd}
	err := fsl.PostService(memfsd.Addr(), name)
	if err != nil {
		return nil, fmt.Errorf("PostService %v error: %v", name, err)
	}
	return fsl, nil
}

func InitFs(name string, memfsd *memfsd.Fsd) (*FsLibSrv, error) {
	fsl := fslib.MakeFsLib(name)
	return InitFsFsl(name, fsl, memfsd)
}

func (fsl *FsLibSrv) ExitFs(name string) {
	err := fsl.Remove(name)
	if err != nil {
		db.DLPrintf("FSCLNT", "Remove failed %v %v\n", name, err)
	}
}
