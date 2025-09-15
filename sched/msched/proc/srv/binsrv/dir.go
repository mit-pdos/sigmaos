package binsrv

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	db "sigmaos/debug"
	"sigmaos/util/perf"
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

// Lookup name in bincache and fake a unix inode
func (n *binFsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	c := ctx.(*fuse.Context).Caller
	db.DPrintf(db.BINSRV, "%v: Lookup %q %d\n", n.path(), name, c.Pid)

	pn := filepath.Join(n.path(), name)

	start := time.Now()

	p, sst, err := n.RootData.bincache.lookup(pn, c.Pid)
	if err != nil {
		return nil, fs.ToErrno(os.ErrNotExist)
	}
	perf.LogSpawnLatencyVerbose("BinSrv.binFsNode.Lookup", p.GetPid(), p.GetSpawnTime(), start)
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

	db.DPrintf(db.BINSRV, "%v: Open pid %d path %q", n, c.Pid, p)

	start := time.Now()
	pr := n.RootData.bincache.pds.LookupProc(int(c.Pid))
	dl := n.RootData.bincache.getDownload(p, n.sz, pr, c.Pid)
	perf.LogSpawnLatencyVerbose("BinSrv.binFsNode.getDownload", dl.p.GetPid(), dl.p.GetSpawnTime(), start)
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
	_, sst, err := n.RootData.bincache.lookup(pn, c.Pid)
	if err != nil {
		return fs.ToErrno(os.ErrNotExist)
	}
	ust := syscall.Stat_t{}
	toUstat(sst, &ust)
	out.Attr.FromStat(&ust)
	return fs.OK
}
