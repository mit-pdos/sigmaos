package binsrv

import (
	"context"
	"fmt"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	db "sigmaos/debug"
)

func newBinFsFile(path string, dl *downloader, st *syscall.Stat_t) fs.FileHandle {
	return &binfsFile{path: path, dl: dl, st: st, fd: -1}
}

type binfsFile struct {
	mu   sync.Mutex
	path string
	n    int
	dl   *downloader
	st   *syscall.Stat_t
	fd   int
}

func (f *binfsFile) String() string {
	return fmt.Sprintf("{F %q st %p dl %p %d}", f.path, f.st, f.dl, f.fd)
}

var _ = (fs.FileHandle)((*binfsFile)(nil))
var _ = (fs.FileReleaser)((*binfsFile)(nil))
var _ = (fs.FileGetattrer)((*binfsFile)(nil))
var _ = (fs.FileReader)((*binfsFile)(nil))
var _ = (fs.FileGetlker)((*binfsFile)(nil))
var _ = (fs.FileLseeker)((*binfsFile)(nil))

func (f *binfsFile) Fd() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.fd
}

func (f *binfsFile) Read(ctx context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {

	fd := f.Fd()
	if fd == -1 {
		fd = f.dl.waitDownload()
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fd == -1 {
		f.fd = fd
	}

	db.DPrintf(db.BINSRV, "Read %q l %d o %d\n", f.path, len(buf), off)

	f.n += len(buf)
	r := fuse.ReadResultFd(uintptr(f.fd), off, len(buf))
	return r, fs.OK
}

func (f *binfsFile) Release(ctx context.Context) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fd != -1 {
		err := syscall.Close(f.fd)
		f.fd = -1
		db.DPrintf(db.BINSRV, "Release %q %d\n", f.path, f.n)
		return fs.ToErrno(err)
	}
	return syscall.EBADF
}

const (
	_OFD_GETLK = 36
)

func (f *binfsFile) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) (errno syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	flk := syscall.Flock_t{}
	lk.ToFlockT(&flk)
	errno = fs.ToErrno(syscall.FcntlFlock(uintptr(f.fd), _OFD_GETLK, &flk))
	out.FromFlockT(&flk)
	return
}

func (f *binfsFile) Getattr(ctx context.Context, a *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.FromStat(f.st)
	return fs.OK
}

func (f *binfsFile) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := unix.Seek(f.fd, int64(off), int(whence))
	return uint64(n), fs.ToErrno(err)
}
