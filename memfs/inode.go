package memfs

import (
	"sync"
	"time"

	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

var makeDir fs.MakeDirF
var path *pathT

type pathT struct {
	sync.Mutex
	path np.Tpath
}

func MakeRootInode(f fs.MakeDirF, ctx fs.CtxI, perm np.Tperm) (fs.FsObj, *np.Err) {
	makeDir = f
	path = &pathT{}
	path.path = np.Tpath(time.Now().Unix())
	return MakeInode(ctx, np.DMDIR, 0, nil)
}

func GenPath() np.Tpath {
	path.Lock()
	defer path.Unlock()
	path.path += 1
	return path.path
}

func MakeInode(ctx fs.CtxI, p np.Tperm, m np.Tmode, parent fs.Dir) (fs.FsObj, *np.Err) {
	i := inode.MakeInode(ctx, p, parent)
	if p.IsDir() {
		return makeDir(i), nil
	} else if p.IsSymlink() {
		return MakeSym(i), nil
	} else if p.IsPipe() {
		return MakePipe(ctx, i), nil
	} else if p.IsFile() || p.IsEphemeral() {
		return MakeFile(i), nil
	} else {
		return nil, np.MkErr(np.TErrInval, p)
	}
}
