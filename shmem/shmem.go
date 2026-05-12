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
