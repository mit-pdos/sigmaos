package binsrv

import (
	"context"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
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

func (n *binFsNode) getDownload(pn string) *downloader {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.dl == nil {
		db.DPrintf(db.BINSRV, "start downloader for %q\n", pn)
		n.dl = newDownloader(pn, n.RootData.Sc, n.RootData.KernelId)
	}
	return n.dl
}

func toUstat(sst *sp.Stat, ust *syscall.Stat_t) {
	const BLOCKSIZE = 4096

	ust.Ino = sst.Qid.Path
	ust.Size = int64(sst.Length)
	ust.Blocks = int64(sst.Length/BLOCKSIZE + 1)
	ust.Atim.Sec = int64(sst.Atime)
	ust.Mtim.Sec = int64(sst.Mtime)
	ust.Ctim.Sec = int64(sst.Mtime)
	ust.Mode = 0777
	ust.Nlink = 2
	//ust.Uid = sst.Uid
	//ust.Gid = sst.Gid
	ust.Blksize = BLOCKSIZE
}

func (n *binFsNode) sStat(pn string, ust *syscall.Stat_t) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	db.DPrintf(db.BINSRV, "%v: sStat %q\n", n, pn)
	paths := downloadPaths(pn, n.RootData.KernelId)
	var r error
	for _, p := range paths {
		sst, err := n.RootData.Sc.Stat(p)
		if err == nil {
			db.DPrintf(db.BINSRV, "%v: Stat %q %v\n", n, p, sst)
			toUstat(sst, ust)
			return nil
		}
		db.DPrintf(db.BINSRV, "%v: Stat %q err %v\n", n, p, err)
		r = err
	}
	return r
}

func (n *binFsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}
	err := syscall.Lstat(p, &st)
	if err != nil {
		if n.sStat(p, &st) != nil {
			return nil, fs.ToErrno(err)
		}
	}
	out.Attr.FromStat(&st)
	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))

	db.DPrintf(db.BINSRV, "%v: Lookup %q %v\n", n, name, node)

	return ch, 0
}

var _ = (fs.NodeOpener)((*binFsNode)(nil))

func (n *binFsNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	p := n.path()

	db.DPrintf(db.BINSRV, "%v: Open %q\n", n, p)

	dl := n.getDownload(p)
	lf := newBinFsFile(p, dl, n.st)
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

	db.DPrintf(db.BINSRV, "%v: Getattr: %v\n", n, f)

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
