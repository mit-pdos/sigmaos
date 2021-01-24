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
	memfsd *memfsd.Fsd
	srv    *npsrv.NpServer
}

func InitFs(name string, dev memfs.Dev) (*FsLibSrv, error) {
	memfsd := memfsd.MakeFsd(false)
	srv := npsrv.MakeNpServer(memfsd, ":0", false)
	fls := &FsLibSrv{MakeFsLib(false), memfsd, srv}
	err := fls.Remove(name)
	if err != nil {
		db.DPrintf("Remove failed %v %v\n", name, err)
	}
	fs := fls.memfsd.Root()
	_, err = fs.MkNod(fs.RootInode(), "dev", dev)
	if err != nil {
		log.Fatal("Create error: dev: ", err)
	}
	srvname := fls.srv.MyAddr()
	err = fls.Symlink(srvname+":pubkey:sharder", name, 0777)
	if err != nil {
		return nil, fmt.Errorf("Symlink %v error: %v\n", name, err)
	}
	return fls, nil
}
