package memfs

import (
	"fmt"
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

func MakeRootInode(f fs.MakeDirF, owner string, perm np.Tperm) (fs.FsObj, error) {
	makeDir = f
	path = &pathT{}
	path.path = np.Tpath(time.Now().Unix())
	return MakeInode(owner, np.DMDIR, 0, nil)
}

func GenPath() np.Tpath {
	path.Lock()
	defer path.Unlock()
	path.path += 1
	return path.path
}

func MakeInode(uname string, p np.Tperm, m np.Tmode, parent fs.Dir) (fs.FsObj, error) {
	i := inode.MakeInode(uname, p, parent)
	if p.IsDir() {
		return makeDir(i), nil
	} else if p.IsSymlink() {
		return MakeSym(i), nil
	} else if p.IsPipe() {
		return MakePipe(i), nil
	} else if p.IsFile() || p.IsEphemeral() {
		return MakeFile(i), nil
	} else {
		return nil, fmt.Errorf("MakeInode: Unknown inode type")
	}
}
