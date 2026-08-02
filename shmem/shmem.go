package shmem

// #include <sys/mman.h>
// #include <fcntl.h>
// #include <errno.h>
// #include <stdlib.h>
// #include <string.h>
//
// static int _shm_open(const char *name, int oflag, mode_t mode, int *err) {
//     int fd = shm_open(name, oflag, mode);
//     if (fd == -1) {
//         *err = errno;
//     }
//     return fd;
// }
//
// static int _shm_unlink(const char *name, int *err) {
//     int r = shm_unlink(name);
//     if (r == -1) {
//         *err = errno;
//     }
//     return r;
// }
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/proc"
)

// Where POSIX shared memory objects live; the tmpfs whose free space bounds
// every segment on the node.
const shmDir = "/dev/shm"

type Segment struct {
	idStr string
	size  int // Segment size, in bytes
	fd    int // File descriptor for POSIX shared memory
	buf   []byte
}

// shm_open wrapper using cgo
func shmOpen(name string, oflag int, mode uint32) (fd int, err error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var cerrno C.int
	r := C._shm_open(cname, C.int(oflag), C.mode_t(mode), &cerrno)
	if r == -1 {
		return -1, syscall.Errno(cerrno)
	}
	return int(r), nil
}

// shm_unlink wrapper using cgo
func shmUnlink(name string) error {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var cerrno C.int
	r := C._shm_unlink(cname, &cerrno)
	if r == -1 {
		return syscall.Errno(cerrno)
	}
	return nil
}

// Open (and optionally create) a shared memory segment
func NewSegment(idStr string, size proc.Tmem, create bool) (*Segment, error) {
	sms := &Segment{
		idStr: idStr,
		size:  int(size),
	}
	// Create POSIX shared memory object with name based on idStr
	name := "/" + idStr
	flags := unix.O_RDWR
	if create {
		flags |= unix.O_CREAT | unix.O_EXCL
	}
	fd, err := shmOpen(name, flags, 0666)
	if err != nil {
		db.DPrintf(db.ERROR, "Err shm_open: %v", err)
		return nil, fmt.Errorf("err shm_open: %v", err)
	}
	sms.fd = fd
	if create {
		// Set the size of the shared memory object
		if err := unix.Ftruncate(fd, int64(size)); err != nil {
			db.DPrintf(db.ERROR, "Err ftruncate: %v", err)
			unix.Close(fd)
			shmUnlink(name)
			return nil, fmt.Errorf("err ftruncate: %v", err)
		}
		// ftruncate and mmap only reserve address space: /dev/shm is a tmpfs
		// whose size is fixed when the container starts (see dcontainer.go) and
		// is shared by every proc on the node, so an oversubscribed tmpfs would
		// otherwise surface as a fault on some later write, far from the cause.
		// Check the free space rather than reserving the pages with fallocate:
		// reserving 139MB costs ~60ms on the proc-spawn path (statfs costs ~4us),
		// which is several times the whole exec->main window. The check races
		// with other procs allocating, so it catches the gross case only; a
		// segment that is merely too small for its own proc still fails loudly in
		// the allocator (see alloc.go).
		var st unix.Statfs_t
		if err := unix.Statfs(shmDir, &st); err != nil {
			db.DPrintf(db.SHMEM, "Statfs %v err %v; skipping free-space check", shmDir, err)
		} else if avail := uint64(st.Bavail) * uint64(st.Bsize); avail < uint64(size) {
			db.DPrintf(db.ERROR, "Err shmem segment %v (%v bytes) larger than free space in %v (%v bytes) — /dev/shm too small for the segments this node's procs request", name, size, shmDir, avail)
			unix.Close(fd)
			shmUnlink(name)
			return nil, fmt.Errorf("err shmem %v: %v bytes requested, %v free in %v", name, size, avail, shmDir)
		}
	}
	// Map the shared memory object into the process address space
	buf, err := unix.Mmap(fd, 0, sms.size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		db.DPrintf(db.ERROR, "Err mmap: %v", err)
		unix.Close(fd)
		shmUnlink(name)
		return nil, fmt.Errorf("err mmap: %v", err)
	}
	sms.buf = buf
	db.DPrintf(db.SHMEM, "Create shmem buffer [%v] at 0x%p sz:%v", sms.idStr, &sms.buf[0], size)
	return sms, nil
}

// Retrieve the buffer referring to a shared memory segment
func (sms *Segment) GetBuf() []byte {
	return sms.buf
}

// Name identifies the segment (the owning proc's PID), for error reporting.
func (sms *Segment) Name() string {
	return sms.idStr
}

// Destroy a shared memory segment
func (sms *Segment) Destroy() error {
	// Unmap the shared memory
	if err := unix.Munmap(sms.buf); err != nil {
		db.DPrintf(db.ERROR, "Err munmap: %v", err)
		return fmt.Errorf("err munmap: %v", err)
	}
	sms.buf = nil
	// Close the file descriptor
	if err := unix.Close(sms.fd); err != nil {
		db.DPrintf(db.ERROR, "Err close: %v", err)
		return fmt.Errorf("err close: %v", err)
	}
	// Unlink the shared memory object
	name := "/" + sms.idStr
	db.DPrintf(db.SHMEM, "Unlink shm file %s", name)
	if err := shmUnlink(name); err != nil && err != syscall.ENOENT {
		db.DPrintf(db.ERROR, "Err shm_unlink: %v", err)
		return fmt.Errorf("err shm_unlink: %v", err)
	}
	return nil
}
