package shmem

import (
	"sync/atomic"

	db "sigmaos/debug"
	"sigmaos/malloc"
)

type Allocator struct {
	s   *Segment
	off atomic.Uint64
}

func NewAllocator(s *Segment) malloc.Allocator {
	return &Allocator{
		s: s,
	}
}

func (a *Allocator) Alloc(b *[]byte, sz int) {
	// Atomically claim the next sz bytes
	endOff := a.off.Add(uint64(sz))
	shmemBuf := a.s.GetBuf()
	// The allocator never reuses or wraps, so running past the end means the
	// segment was sized too small for everything this proc will have delegated
	// into it. Name the segment and the shortfall: the size is chosen per proc
	// by its parent (for MR, mapperShmemMB/reducerShmemMB in apps/mr/coord,
	// or the job description's reduce_shmem_mb).
	if int(endOff) >= len(shmemBuf) {
		db.DFatalf("Err shmem segment %v too small: allocating %v bytes needs %v, have %v — increase the proc's ShmemMB",
			a.s.Name(), sz, endOff, len(shmemBuf))
	}
	// Calculate offset to the start of the buffer
	startOff := int(endOff) - sz
	// Set the buffer to point into the shared memory segment
	*b = shmemBuf[startOff:endOff]
	db.DPrintf(db.SHMEM, "Shmem Alloc 0x%p sz %v", *b, sz)
}
