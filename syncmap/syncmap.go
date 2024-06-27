package syncmap

import (
	"sync"
)

//
// Thread-safe map
//

type SyncMap[K comparable, T any] struct {
	sync.Mutex
	tbl map[K]T
}

func NewSyncMap[K comparable, T any]() *SyncMap[K, T] {
	return &SyncMap[K, T]{tbl: make(map[K]T)}
}

func (sm *SyncMap[_, _]) Len() int {
	sm.Lock()
	defer sm.Unlock()
	return len(sm.tbl)
}

func (sm *SyncMap[K, T]) Lookup(k K) (T, bool) {
	sm.Lock()
	defer sm.Unlock()
	r, ok := sm.tbl[k]
	return r, ok
}

// Returns true if allocated a new entry for k
func (sm *SyncMap[K, T]) Alloc(k K, ne T) (T, bool) {
	sm.Lock()
	defer sm.Unlock()
	if e, ok := sm.tbl[k]; ok {
		return e, false
	}
	sm.tbl[k] = ne
	return ne, true
}

// If k doesn't exist, call ne while holding syncmap's lock. It
// returns true if allocated a new entry for k
func (sm *SyncMap[K, T]) AllocNew(k K, ne func(k K) T) (T, bool) {
	sm.Lock()
	defer sm.Unlock()
	if e, ok := sm.tbl[k]; ok {
		return e, false
	}
	sm.tbl[k] = ne(k)
	return sm.tbl[k], true
}

func (sm *SyncMap[K, T]) Insert(k K, t T) bool {
	sm.Lock()
	defer sm.Unlock()

	if _, ok := sm.tbl[k]; ok {
		return false
	}
	sm.tbl[k] = t
	return true
}

func (sm *SyncMap[K, T]) Update(k K, t T) bool {
	sm.Lock()
	defer sm.Unlock()

	if _, ok := sm.tbl[k]; !ok {
		return false
	}
	sm.tbl[k] = t
	return true
}

func (sm *SyncMap[K, _]) Delete(k K) {
	sm.Lock()
	defer sm.Unlock()

	if _, ok := sm.tbl[k]; ok {
		delete(sm.tbl, k)
	}
}

func (sm *SyncMap[K, T]) LookupDelete(k K) (T, bool) {
	sm.Lock()
	defer sm.Unlock()

	if v, ok := sm.tbl[k]; ok {
		delete(sm.tbl, k)
		return v, true
	} else {
		return v, false
	}
}

func (sm *SyncMap[K, T]) Rename(src, dst K) {
	sm.Lock()
	defer sm.Unlock()

	if val, ok := sm.tbl[src]; ok {
		delete(sm.tbl, src)
		sm.tbl[dst] = val
	}
}

func (sm *SyncMap[K, T]) Values() []T {
	sm.Lock()
	defer sm.Unlock()

	vals := make([]T, len(sm.tbl))
	i := 0
	for _, v := range sm.tbl {
		vals[i] = v
		i += 1
	}
	return vals
}

func (sm *SyncMap[K, V]) Iter(f func(key K, val V)) {
	sm.Lock()
	defer sm.Unlock()

	for k, v := range sm.tbl {
		f(k, v)
	}
}
