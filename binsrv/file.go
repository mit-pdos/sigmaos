package binsrv

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	db "sigmaos/debug"
)

type binfsFile struct {
	mu sync.Mutex
	pn string
	n  int
	dl *downloader
	fd int
}

func newBinFsFile(pn string, dl *downloader) fs.FileHandle {
	return &binfsFile{pn: pn, dl: dl, fd: -1}
}

func (f *binfsFile) String() string {
	return fmt.Sprintf("{F %q st %p dl %p %d}", f.pn, f.dl, f.fd)
}

var _ = (fs.FileHandle)((*binfsFile)(nil))
var _ = (fs.FileReleaser)((*binfsFile)(nil))
var _ = (fs.FileReader)((*binfsFile)(nil))
var _ = (fs.FileGetlker)((*binfsFile)(nil))
var _ = (fs.FileLseeker)((*binfsFile)(nil))

func (f *binfsFile) open() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fd == -1 {
		pn := binCachePath(f.pn)
		fd, err := syscall.Open(pn, syscall.O_RDONLY, 0)
		if err != nil {
			db.DFatalf("open %q err %v", f.pn, err)
		}
		f.fd = fd
	}
}

func (f *binfsFile) Read(ctx context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	db.DPrintf(db.BINSRV, "Read %q off %d len %d", f.pn, off, len(buf))
	start := time.Now()
	sz, err := f.dl.read(off, len(buf))
	if err != nil {
		db.DPrintf(db.BINSRV, "Read %q err %v", f.pn, err)
		return nil, syscall.EBADF
	}
	f.open()
	db.DPrintf(db.BINSRV, "ReadResult %q o %d sz %d", f.pn, off, sz)
	db.DPrintf(db.SPAWN_LAT, "FUSE.Read latency %q o %d sz %d: %v", f.pn, off, sz, time.Since(start))
	r := fuse.ReadResultFd(uintptr(f.fd), off, int(sz))
	return r, fs.OK
}

func (f *binfsFile) Release(ctx context.Context) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fd != -1 {
		err := syscall.Close(f.fd)
		f.fd = -1
		db.DPrintf(db.BINSRV, "Release %q %d\n", f.pn, f.n)
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

func (f *binfsFile) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := unix.Seek(f.fd, int64(off), int(whence))
	return uint64(n), fs.ToErrno(err)
}
