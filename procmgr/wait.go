package procmgr

import (
	"sync"
)

type Waiter struct {
	c    *sync.Cond
	done bool
}

func newWaiter(mu *sync.Mutex) *Waiter {
	return &Waiter{
		c:    sync.NewCond(mu),
		done: false,
	}
}

// Caller holds lock.
func (w *Waiter) wait() {
	if w.done {
		return
	}
	w.c.Wait()
}

// Caller holds lock.
func (w *Waiter) release() {
	w.done = true
	w.c.Broadcast()
}
