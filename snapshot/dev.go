package snapshot

import (
	"log"

	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type Dev struct {
	fs.Inode
	srv np.FsServer
}

func MakeDev(srv np.FsServer, ctx fs.CtxI, root fs.Dir) *Dev {
	i := inode.MakeInode(ctx, 0, root)
	return &Dev{i, srv}
}

func (dev *Dev) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	b := dev.srv.Snapshot()
	if len(b) > int(np.MAXGETSET) {
		log.Fatalf("FATAL snapshot too big: %v bytes", len(b))
	}
	return b, nil
}

func (dev *Dev) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	log.Printf("Received snapshot of length %v", len(b))
	dev.srv.Restore(b)
	return np.Tsize(len(b)), nil
}

func (dev *Dev) Snapshot(fn fs.SnapshotF) []byte {
	return dev.Inode.Snapshot(fn)
}

func RestoreSnapshotDev(fn fs.RestoreF, data []byte) fs.Inode {
	d := &Dev{}
	d.Inode = inode.RestoreInode(fn, data)
	return d
}
