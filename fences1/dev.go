package fences1

import (
	// "encoding/json"
	"log"
	"sync"

	"ulambda/ctx"
	// db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

const NAME = "fenced"

type FencesDev struct {
	sync.Mutex
	fs.Inode
}

func MkFencesDev(parent fs.Dir) {
	fd := &FencesDev{}
	fd.Inode = inode.MakeInode(nil, np.DMDEVICE, parent)
	if err := dir.MkNod(ctx.MkCtx("", 0, nil), parent, NAME, fd); err != nil {
		log.Fatalf("FATAL MkFencesDev %v\n", err)
	}
}

func (fd *FencesDev) Read(ctx fs.CtxI, off np.Toffset, n np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	log.Printf("fencesdev read\n")
	return nil, nil
}

func (fd *FencesDev) Write(ctx fs.CtxI, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, nil
}
