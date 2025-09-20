package shmem

import (
	"sync/atomic"

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
	// Calculate offset to the start of the buffer
	startOff := int(endOff) - sz
	// Set the buffer to point into the shared memory segment
	*b = a.s.GetBuf()[startOff:endOff]
}
