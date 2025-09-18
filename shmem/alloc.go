package shmem

import (
	db "sigmaos/debug"
)

var ALLOCATOR *Allocator

type Allocator struct {
	seg     *Segment
	lastOff int
}

func NewAllocator(seg *Segment) *Allocator {
	return &Allocator{
		seg: seg,
	}
}

func ALLOC_BUF(nbyte int) []byte {
	db.DPrintf(db.SHMEM, "Alloc buf nbyte %v", nbyte)
	off := ALLOCATOR.lastOff
	ALLOCATOR.lastOff += nbyte
	return ALLOCATOR.seg.GetBuf()[off : off+nbyte]
}
