package freelist

import (
	"sync"

	db "sigmaos/debug"
)

type FreeList[T any] struct {
	mu       sync.Mutex
	freelist []*T
	nNew     int
}

func NewFreeList[T any](sz int) *FreeList[T] {
	return &FreeList[T]{freelist: make([]*T, 0, sz)}
}

func (fl *FreeList[T]) Len() int {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	db.DPrintf(db.TEST, "New: newNew %d", fl.nNew)

	return len(fl.freelist)
}

func (fl *FreeList[T]) New() *T {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	index := len(fl.freelist) - 1
	if index < 0 {
		db.DPrintf(db.TEST, "New: allocate %d", index)
		fl.nNew += 1
		return new(T)
	}
	e := fl.freelist[index]
	fl.freelist = fl.freelist[:index]
	return e
}

func (fl *FreeList[T]) Free(e *T) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if len(fl.freelist) < cap(fl.freelist) {
		fl.freelist = append(fl.freelist, e)
	}
}
