package memfs

import (
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

var makeDir fs.MakeDirF

func MakeRootInode(f fs.MakeDirF, owner string, perm np.Tperm) (fs.FsObj, error) {
	makeDir = f
	return MakeInode(owner, np.DMDIR, 0, nil)
}

func MakeInode(uname string, p np.Tperm, m np.Tmode, parent fs.Dir) (fs.FsObj, error) {
	i := inode.MakeInode(uname, p, parent)
	if p.IsDir() {
		return makeDir(i), nil
	} else if p.IsSymlink() {
		return MakeSym(i), nil
	} else if p.IsPipe() {
		return MakePipe(i), nil
	} else if p.IsDevice() {
		return MakeDev(i), nil
	} else {
		return MakeFile(i), nil
	}
}

func MkNod(ctx fs.CtxI, dir fs.Dir, name string, dev Dev) error {
	di, err := dir.Create(ctx, name, np.DMDEVICE, 0)
	if err != nil {
		return err
	}
	d := di.(*Device)
	d.d = dev
	return nil
}
