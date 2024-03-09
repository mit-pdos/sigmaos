package binsrv

import (
	"context"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	db "sigmaos/debug"
)

func newBinFsFile(fd int, path string) fs.FileHandle {
	return &binfsFile{fd: fd, path: path}
}

type binfsFile struct {
	mu   sync.Mutex
	fd   int
	path string
	n    int
}

var _ = (fs.FileHandle)((*binfsFile)(nil))
var _ = (fs.FileReleaser)((*binfsFile)(nil))
var _ = (fs.FileGetattrer)((*binfsFile)(nil))
var _ = (fs.FileReader)((*binfsFile)(nil))
var _ = (fs.FileGetlker)((*binfsFile)(nil))
var _ = (fs.FileLseeker)((*binfsFile)(nil))

func (f *binfsFile) Read(ctx context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
		db.DPrintf(db.BINSRV, "read %q %d\n", f.path, f.n)
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
	st := syscall.Stat_t{}
	err := syscall.Fstat(f.fd, &st)
	if err != nil {
		return fs.ToErrno(err)
	}
	a.FromStat(&st)

	return fs.OK
}

func (f *binfsFile) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := unix.Seek(f.fd, int64(off), int(whence))
	return uint64(n), fs.ToErrno(err)
}
