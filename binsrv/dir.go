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

	st := syscall.Stat_t{}
	err := syscall.Lstat(pn, &st)
	if err == nil {
		db.DPrintf(db.BINSRV, "getDownload: %q present\n", pn)
		n.dl = newDownloaderPresent(pn, n.RootData.Sc, n.RootData.KernelId)
		return n.dl
	}

	if n.dl == nil {
		db.DPrintf(db.BINSRV, "getDownload: new downloader %q\n", pn)
		n.dl = newDownloader(pn, n.RootData.Sc, n.RootData.KernelId)
	}
	return n.dl
}

func idFromStat(st *syscall.Stat_t) fs.StableAttr {
	swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		Ino:  swapped ^ st.Ino,
	}
}

func toUstat(sst *sp.Stat, ust *syscall.Stat_t) {
	const BLOCKSIZE = 4096

	ust.Dev = uint64(sst.Dev)
	ust.Ino = sst.Qid.Path
	ust.Size = int64(sst.Length)
	ust.Blocks = int64(sst.Length/BLOCKSIZE + 1)
	ust.Atim.Sec = int64(sst.Atime)
	ust.Mtim.Sec = int64(sst.Mtime)
	ust.Ctim.Sec = int64(sst.Mtime)
	ust.Mode = 0777
	ust.Nlink = 1
	ust.Blksize = BLOCKSIZE
}

func (n *binFsNode) sStat(pn string, ust *syscall.Stat_t) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	db.DPrintf(db.BINSRV, "%v: sStat %q\n", n, pn)
	paths := downloadPaths(pn, n.RootData.KernelId)
	return retryPaths(paths, func(i int, pn string) error {
		sst, err := n.RootData.Sc.Stat(pn)
		if err == nil {
			sst.Dev = uint32(i)
			toUstat(sst, ust)
			return nil
		}
		return err
	})
}

// Check cache first. If not present, Stat file in sigmaos and create
// a fake unix inode.
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
	ch := n.NewInode(ctx, node, idFromStat(&st))

	db.DPrintf(db.BINSRV, "%v: Lookup %q %v\n", n, name, node)

	return ch, 0
}

var _ = (fs.NodeOpener)((*binFsNode)(nil))

// Open a file in the cache. It returns a binFsFile with a download
// obj.  If the file isn't present in cache, getDownload starts a
// downloader and returns download object for others to wait on.  If
// the file is in the cache, the download object is marked done.
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
