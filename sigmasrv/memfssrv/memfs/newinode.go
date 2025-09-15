package memfs

import (
	"sigmaos/api/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
)

type NewInode struct {
	ia *inode.InodeAlloc
}

func NewNewInode(dev sp.Tdev) *NewInode {
	return &NewInode{ia: inode.NewInodeAlloc(dev)}
}

func (ni *NewInode) InodeAlloc() *inode.InodeAlloc {
	return ni.ia
}

func (ni *NewInode) NewFsObj(ctx fs.CtxI, p sp.Tperm, lid sp.TleaseId, m sp.Tmode, nd fs.MkDirF) (fs.FsObj, *serr.Err) {
	i := ni.ia.NewInode(ctx, p, lid)
	if p.IsDir() {
		return nd(i, ni), nil
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
