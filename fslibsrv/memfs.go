package fslibsrv

import (
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/inode"
	np "sigmaos/ninep"
	"sigmaos/procclnt"
	"sigmaos/sesssrv"
)

type MemFs struct {
	*fslib.FsLib
	*sesssrv.SessSrv
	procclnt *procclnt.ProcClnt
	root     fs.Dir
	ctx      fs.CtxI // server context
}

func (fs *MemFs) Root() fs.Dir {
	return fs.root
}

func (mfs *MemFs) nameiParent(path np.Path) (fs.Dir, *np.Err) {
	d := mfs.root
	if len(path) > 1 {
		_, o, _, err := mfs.root.LookupPath(mfs.ctx, path.Dir())
		if err != nil {
			return nil, err
		}
		d = o.(fs.Dir)
	}
	return d, nil
}

// Caller must store i in dev.Inode
func (mfs *MemFs) MkDev(pn string, dev fs.Inode) (*inode.Inode, *np.Err) {
	path := np.Split(pn)
	d, err := mfs.nameiParent(path)
	if err != nil {
		return nil, err
	}
	i := inode.MakeInode(mfs.ctx, np.DMDEVICE, d)
	return i, dir.MkNod(mfs.ctx, d, path.Base(), dev)
}

// XXX handle d being removed between lookup and create?
func (mfs *MemFs) Create(pn string, p np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	path := np.Split(pn)
	d, err := mfs.nameiParent(path)
	if err != nil {
		return nil, err
	}
	return d.Create(mfs.ctx, path.Base(), p, m)
}

func (mfs *MemFs) RemoveXXX(pn string) *np.Err {
	path := np.Split(pn)
	d, err := mfs.nameiParent(path)
	if err != nil {
		return err
	}
	return d.Remove(mfs.ctx, path.Base())
}
