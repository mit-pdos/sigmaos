package snapshot

import (
	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/fs"
	"sigmaos/inode"
	sp "sigmaos/sigmap"
)

type Dev struct {
	fs.Inode
	srv sp.SessServer
}

func MakeDev(srv sp.SessServer, ctx fs.CtxI, root fs.Dir) *Dev {
	i := inode.MakeInode(ctx, 0, root)
	return &Dev{i, srv}
}

func (dev *Dev) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	b := dev.srv.Snapshot()
	if len(b) > int(sp.MAXGETSET) {
		db.DFatalf("snapshot too big: %v bytes", len(b))
	}
	return b, nil
}

func (dev *Dev) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	db.DPrintf(db.SNAP, "Received snapshot of length %v", len(b))
	dev.srv.Restore(b)
	return sessp.Tsize(len(b)), nil
}

func (dev *Dev) Snapshot(fn fs.SnapshotF) []byte {
	return dev.Inode.Snapshot(fn)
}

func RestoreSnapshotDev(fn fs.RestoreF, data []byte) fs.Inode {
	d := &Dev{}
	d.Inode = inode.RestoreInode(fn, data)
	return d
}
