package snapshot

import (
	"log"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

type Dev struct {
	fs.FsObj
	srv protsrv.FsServer
}

func MakeDev(srv protsrv.FsServer, ctx fs.CtxI, root fs.Dir) *Dev {
	i := inode.MakeInode(ctx, 0, root)
	dev := &Dev{i, srv}
	dir.MkNod(ctx, root, "snapshot", dev)
	return dev
}

func (dev *Dev) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	log.Printf("Sent snapshot")
	return dev.srv.Snapshot(), nil
}

func (dev *Dev) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	log.Printf("Received snapshot of length %v", len(b))
	return np.Tsize(len(b)), nil
}
