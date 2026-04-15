package pyenv

import (
	"container/list"
	"sync"
	"sync/atomic"
)

type zeroListItem struct {
	rc   refCount
	elem atomic.Pointer[list.Element]
}

type zeroList struct {
	mu sync.Mutex
	l  list.List
}

func newZeroList() zeroList {
	return zeroList{
		l: *list.New(),
	}
}

func (z *zeroList) addIfZero(it *zeroListItem) {
	// fast-path: already present
	if it.elem.Load() != nil {
		return
	}

	z.mu.Lock()
	defer z.mu.Unlock()

	// Close race: if it became active or closed, don't add.
	if it.rc.state.Load() != 0 {
		return
	}
	// Duplicate race: someone else added while we waited.
	if it.elem.Load() != nil {
		return
	}

	e := z.l.PushBack(it)
	it.elem.Store(e)
}

func (z *zeroList) removeIfNonZero(it *zeroListItem) {
	e := it.elem.Load()
	if e == nil {
		return
	}

	z.mu.Lock()
	defer z.mu.Unlock()

	e = it.elem.Load()
	if e == nil {
		return
	}

	if it.rc.state.Load() == 0 {
		return
	}

	z.l.Remove(e)
	it.elem.Store(nil)
}

func (it *zeroListItem) acquire(z *zeroList) error {
	newv, err := it.rc.acquire()
	if err != nil {
		return err
	}
	if newv == 1 {
		z.removeIfNonZero(it)
	}
	return nil
}

// Return true if the refcount reached zero
func (it *zeroListItem) release(z *zeroList) bool {
	newv := it.rc.release()
	if newv == 0 {
		z.addIfZero(it)
	}
	return newv == 0
}

func (it *zeroListItem) tryClose(z *zeroList) bool {
	if !it.rc.tryClose() {
		return false
	}

	// If close succeeded, the refcount will be -1
	z.removeIfNonZero(it)
	return true
}
