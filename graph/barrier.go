package graph

// Imported from https://github.com/Leimy/Go-Barrier

import (
	"sync"
)

// A Barrier is a synchronization mechanism for groups of worker goroutines.
// Barriers are used to synchronize that group of goroutines to different points
// in a parallel algorithm.  For example, many goroutines may be assigned
// ownership of parts of an array or slice for updating in parallel, and another
// goroutine may wish to synchronize across the entire group to get the results.
//
// For a more flexible interface you can use WaitGroup from the standard Go sync
// package.
type Barrier struct {
	n      int
	l      *sync.Mutex
	waiter *sync.RWMutex
}

// NewBarrier creates a new barrier of size N.  Subsequent calls to Wait will
// block all goroutines that have called it until Wait is called N times.
func NewBarrier(n int) *Barrier {
	if n <= 0 {
		panic("Group must be >= 1")
	}
	waiter := &sync.RWMutex{}
	waiter.Lock()
	return &Barrier{n, &sync.Mutex{}, waiter}
}

// Wait blocks until all the members of the group (size N from NewBarrier) have
// "checked in".
func (b *Barrier) Wait() {
	b.l.Lock()
	b.n--
	if b.n == 0 {
		b.waiter.Unlock()
	}
	b.l.Unlock()
	b.waiter.RLock()
}
