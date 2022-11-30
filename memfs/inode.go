package memfs

import (
	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
)

func MakeInode(ctx fs.CtxI, p np.Tperm, m np.Tmode, parent fs.Dir, mk fs.MakeDirF) (fs.Inode, *fcall.Err) {
	i := inode.MakeInode(ctx, p, parent)
	if p.IsDir() {
		return mk(i, MakeInode), nil
	} else if p.IsSymlink() {
		return MakeFile(i), nil
	} else if p.IsPipe() {
		return MakePipe(ctx, i), nil
	} else if p.IsFile() || p.IsEphemeral() {
		return MakeFile(i), nil
	} else {
		return nil, fcall.MkErr(fcall.TErrInval, p)
	}
}
