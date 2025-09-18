package shmem

import (
	"fmt"
	"hash/fnv"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
)

var SHMEM_BUF []byte
var SEGMENT *Segment

type Segment struct {
	idStr    string
	key      uint32 // User-specified key
	size     int    // Segment size
	id       int    // ID returned by shmem creation
	attached bool
	buf      []byte
}

// Create a shared memory segment
func NewSegment(idStr string, size int) (*Segment, error) {
	sms := &Segment{
		idStr: idStr,
		key:   id2key(idStr),
		size:  size,
	}
	if sms.key == unix.IPC_PRIVATE {
		db.DPrintf(db.ERROR, "Err IPC private idStr %v", idStr)
		return nil, fmt.Errorf("err IPC private idStr %v", idStr)
	}
	id, err := unix.SysvShmGet(int(sms.key), size, unix.IPC_CREAT|unix.IPC_EXCL|0666)
	if err != nil {
		db.DPrintf(db.ERROR, "Err shmget: %v", err)
		return nil, fmt.Errorf("err shmget: %v", err)
	}
	sms.id = id
	sms.buf, err = unix.SysvShmAttach(sms.id, 0, 0)
	if err != nil {
		db.DPrintf(db.ERROR, "Err shmat: %v", err)
		return nil, fmt.Errorf("err shmat: %v", err)
	}
	db.DPrintf(db.SHMEM, "Create shmem buffer key [%v] -> [%v] at 0x%p", sms.idStr, sms.key, &sms.buf[0])
	return sms, nil
}

// Retrieve the buffer referring to a shared memory segment
func (sms *Segment) GetBuf() []byte {
	return sms.buf
}

// Destroy a shared memory segment
func (sms *Segment) Destroy() error {
	if err := unix.SysvShmDetach(sms.buf); err != nil {
		db.DPrintf(db.ERROR, "Err shm detach: %v", err)
		return fmt.Errorf("err shm detach: %v", err)
	}
	sms.buf = nil
	_, err := unix.SysvShmCtl(sms.id, unix.IPC_RMID, nil)
	if err != nil {
		db.DPrintf(db.ERROR, "Err shm destroy: %v", err)
		return fmt.Errorf("err shm destroy: %v", err)
	}
	return nil
}

func id2key(id string) uint32 {
	h := fnv.New64a()
	h.Write([]byte(id))
	return uint32(h.Sum64())
}
