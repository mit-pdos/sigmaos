package binsrv

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	db "sigmaos/debug"
)

var _ = (fs.NodeStatfser)((*binFsNode)(nil))

func (n *binFsNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	const BLOCKSIZE = 4096
	s := syscall.Statfs_t{Bsize: BLOCKSIZE, Blocks: 1000}
	db.DPrintf(db.BINSRV, "Statfs %v\n", n.path())
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

// Lookup name in bincache and fake a unix inode
func (n *binFsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	c := ctx.(*fuse.Context).Caller
	db.DPrintf(db.BINSRV, "%v: Lookup %q %d\n", n.path(), name, c.Pid)

	pn := filepath.Join(n.path(), name)

	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup pid %v", pn, c.Pid)

	sst, err := n.RootData.bincache.lookup(pn, c.Pid)
	if err != nil {
		return nil, fs.ToErrno(os.ErrNotExist)
	}
	ust := syscall.Stat_t{}
	toUstat(sst, &ust)
	out.Attr.FromStat(&ust)
	node := n.RootData.newNode(n.EmbeddedInode(), name, sst.Tsize())
	ch := n.NewInode(ctx, node, idFromStat(&ust))

	db.DPrintf(db.BINSRV, "%v: Lookup done name %q node %v\n", n, name, node)

	return ch, 0
}

var _ = (fs.NodeOpener)((*binFsNode)(nil))

// Open returns a binFsFile with a download obj.
func (n *binFsNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	p := n.path()

	c := ctx.(*fuse.Context).Caller
	db.DPrintf(db.SPAWN_LAT, "[%v] Open pid %d", p, c.Pid)

	db.DPrintf(db.BINSRV, "%v: Open path %q", n, p)

	dl := n.RootData.bincache.getDownload(p, n.sz, c.Pid)
	lf := newBinFsFile(p, dl)
	return lf, fuse.FOPEN_KEEP_CACHE, 0
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
	c := ctx.(*fuse.Context).Caller
	pn := n.path()
	sst, err := n.RootData.bincache.lookup(pn, c.Pid)
	if err != nil {
		return fs.ToErrno(os.ErrNotExist)
	}
	ust := syscall.Stat_t{}
	toUstat(sst, &ust)
	out.Attr.FromStat(&ust)
	return fs.OK
}
