package syncmap

import (
	"sync"
)

type SyncMap[K comparable, T any] struct {
	sync.Mutex
	tbl map[K]T
}

func NewSyncMap[K comparable, T any]() *SyncMap[K, T] {
	return &SyncMap[K, T]{tbl: make(map[K]T)}
}

func (sm *SyncMap[K, T]) Lookup(k K) (T, bool) {
	sm.Lock()
	defer sm.Unlock()
	r, ok := sm.tbl[k]
	return r, ok
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
