package memfs

import (
	"sigmaos/fs"
	"sigmaos/memfs/inode"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func NewInode(ctx fs.CtxI, p sp.Tperm, lid sp.TleaseId, m sp.Tmode, parent fs.Dir, new fs.MkDirF) (fs.FsObj, *serr.Err) {
	i := inode.NewInode(ctx, p, lid, parent)
	if p.IsDir() {
		return new(i, NewInode), nil
	} else if p.IsSymlink() {
		return NewFile(i), nil
	} else if p.IsPipe() {
		return NewPipe(ctx, i), nil
	} else if p.IsFile() {
		return NewFile(i), nil
	} else {
		return nil, serr.NewErr(serr.TErrInval, p)
	}
}
