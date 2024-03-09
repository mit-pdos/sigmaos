package binsrv

import (
	"context"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	db "sigmaos/debug"
	// sp "sigmaos/sigmap"
)

var _ = (fs.NodeStatfser)((*binFsNode)(nil))

func (n *binFsNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	s := syscall.Statfs_t{}
	err := syscall.Statfs(n.path(), &s)
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStatfsT(&s)
	return fs.OK
}

// path returns the full path to the file in the underlying file
// system.
func (n *binFsNode) path() string {
	path := n.Path(n.Root())
	return filepath.Join(n.RootData.Path, path)
}

var _ = (fs.NodeLookuper)((*binFsNode)(nil))

func (n *binFsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}
	err := syscall.Lstat(p, &st)
	if err != nil {
		if err := n.downloadProcBin(p); err != nil {
			db.DPrintf(db.BINSRV, "download %q err %v\n", p, err)
		}
		if err := syscall.Lstat(p, &st); err != nil {
			return nil, fs.ToErrno(err)
		}
	}
	out.Attr.FromStat(&st)
	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))
	return ch, 0
}

var _ = (fs.NodeOpener)((*binFsNode)(nil))

func (n *binFsNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	flags = flags &^ syscall.O_APPEND
	p := n.path()
	f, err := syscall.Open(p, int(flags), 0)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}
	lf := newBinFsFile(f, p)
	return lf, 0, 0
}

var _ = (fs.NodeOpendirer)((*binFsNode)(nil))

func (n *binFsNode) Opendir(ctx context.Context) syscall.Errno {
	fd, err := syscall.Open(n.path(), syscall.O_DIRECTORY, 0755)
	if err != nil {
		return fs.ToErrno(err)
	}
	syscall.Close(fd)
	return fs.OK
}

var _ = (fs.NodeReaddirer)((*binFsNode)(nil))

func (n *binFsNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewLoopbackDirStream(n.path())
}

var _ = (fs.NodeGetattrer)((*binFsNode)(nil))

func (n *binFsNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	if f != nil {
		return f.(fs.FileGetattrer).Getattr(ctx, out)
	}

	p := n.path()

	var err error
	st := syscall.Stat_t{}
	if &n.Inode == n.Root() {
		err = syscall.Stat(p, &st)
	} else {
		err = syscall.Lstat(p, &st)
	}

	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStat(&st)
	return fs.OK
}
