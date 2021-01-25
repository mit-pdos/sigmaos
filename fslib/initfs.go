package fslib

import (
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/memfs"
	"ulambda/memfsd"
	"ulambda/npsrv"
)

type FsLibSrv struct {
	*FsLib
	*memfsd.Fsd
	srv *npsrv.NpServer
}

func InitFsMemFsD(name string, memfs *memfs.Root, memfsd *memfsd.Fsd, dev memfs.Dev) (*FsLibSrv, error) {
	srv := npsrv.MakeNpServer(memfsd, ":0", false)
	fls := &FsLibSrv{MakeFsLib(name), memfsd, srv}
	err := fls.Remove(name)
	if err != nil {
		db.DPrintf("Remove failed %v %v\n", name, err)
	}
	fs := memfsd.Root()
	if dev != nil {
		_, err = fs.MkNod(fs.RootInode(), "dev", dev)
		if err != nil {
			log.Fatal("Create error: dev: ", err)
		}
	}
	srvname := fls.srv.MyAddr()
	log.Printf("srvname %v\n", srvname)
	err = fls.Symlink(srvname+":pubkey:"+name, name, 0777)
	if err != nil {
		return nil, fmt.Errorf("Symlink %v error: %v\n", name, err)
	}
	return fls, nil
}

func InitFsMemFs(name string, memfs *memfs.Root, dev memfs.Dev) (*FsLibSrv, error) {
	memfsd := memfsd.MakeFsd(false, memfs, nil)
	return InitFsMemFsD(name, memfs, memfsd, dev)
}

func InitFs(name string, dev memfs.Dev) (*FsLibSrv, error) {
	fs := memfs.MakeRoot(false)
	fsd := memfsd.MakeFsd(false, fs, nil)
	return InitFsMemFsD(name, fs, fsd, dev)
}
