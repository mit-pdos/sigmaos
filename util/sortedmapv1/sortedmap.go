// Package sortedmap implements a sorted map using a btree.  Keys may
// be present but not have a value yet, which [dircache] uses.
package sortedmapv1

import (
	"fmt"
	"sync"

	"github.com/google/btree"
	"golang.org/x/exp/constraints"
	"sigmaos/util/rand"
)

type kv[K constraints.Ordered, V any] struct {
	present bool // is V present?
	k       K
	v       V
}

func Less[K constraints.Ordered, V any](a, b kv[K, V]) bool {
	return a.k < b.k
}

type SortedMap[K constraints.Ordered, V any] struct {
	sync.Mutex
	tr *btree.BTreeG[kv[K, V]]
	rr kv[K, V]
}

// TODO for SortedMap's that share a free list
type SortedMaps[K constraints.Ordered, V any] struct {
	freelist *btree.FreeListG[kv[K, V]]
}

func NewSortedMapFreeList[K constraints.Ordered, V any]() *SortedMaps[K, V] {
	return nil
}

func (sms *SortedMaps[K, V]) NewSortedMap() {
}

func NewSortedMap[K constraints.Ordered, V any]() *SortedMap[K, V] {
	tr := btree.NewG[kv[K, V]](32, Less)
	return &SortedMap[K, V]{tr: tr}
}

func (sm *SortedMap[K, V]) roundrobinNextL() K {
	sm.tr.AscendGreaterOrEqual(sm.rr, func(e kv[K, V]) bool {
		if sm.rr.k != e.k {
			sm.rr = e
			return false
		} else if sm.tr.Len() > 1 {
			return true
		} else {
			return false
		}
	})
	return sm.rr.k
}

// calls until f returns false
func (sm *SortedMap[K, V]) Iter(f func(key K, v V) bool) {
	sm.Lock()
	defer sm.Unlock()

	sm.tr.Ascend(func(v kv[K, V]) bool {
		return f(v.k, v.v)
	})
}

func (sm *SortedMap[K, V]) String() string {
	s := "["
	sm.Iter(func(k K, v V) bool {
		s += fmt.Sprintf("%v , ", k)
		return true
	})
	return s + "]"
}

func (sm *SortedMap[K, V]) Len() int {
	return sm.tr.Len()
}

func (sm *SortedMap[K, V]) Lookup(n K) (V, bool) {
	sm.Lock()
	defer sm.Unlock()

	e, ok := sm.tr.Get(kv[K, V]{k: n})
	return e.v, ok
}

func (sm *SortedMap[K, V]) LookupKeyKv(k K) (bool, V, bool) {
	sm.Lock()
	defer sm.Unlock()

	e, ok := sm.tr.Get(kv[K, V]{k: k})
	if !ok {
		return false, e.v, false
	}
	if e.present {
		return true, e.v, true
	}
	return true, e.v, false
}

func (sd *SortedMap[K, V]) LookupKeyVal(k K) (bool, V, bool) {
	sd.Lock()
	defer sd.Unlock()

	e, ok := sd.tr.Get(kv[K, V]{k: k})
	if !ok {
		var v V
		return false, v, false
	}
	if e.present {
		return true, e.v, true
	}
	return true, e.v, false
}

func (sm *SortedMap[K, V]) LookupKey(n K) bool {
	sm.Lock()
	defer sm.Unlock()

	_, ok := sm.tr.Get(kv[K, V]{k: n})
	return ok
}

// XXX modify btree to make this log(n)
func (sm *SortedMap[K, V]) Random() (K, bool) {
	sm.Lock()
	defer sm.Unlock()

	if sm.tr.Len() == 0 {
		var k K
		return k, false
	}
	r := rand.Int64(int64(sm.tr.Len()))
	i := uint64(0)
	var k K
	sm.tr.Ascend(func(e kv[K, V]) bool {
		if i == r {
			k = e.k
			return false
		} else {
			i += 1
			return true
		}
	})
	return k, true
}

func (sm *SortedMap[K, V]) RoundRobin() (K, bool) {
	sm.Lock()
	defer sm.Unlock()

	if sm.tr.Len() == 0 {
		var k K
		return k, false
	}
	k := sm.roundrobinNextL()
	return k, true
}

func (sm *SortedMap[K, V]) Keys() []K {
	keys := make([]K, 0)
	sm.Iter(func(k K, v V) bool {
		keys = append(keys, k)
		return true
	})
	return keys
}

// Return true if K was inserted
func (sm *SortedMap[K, V]) InsertKey(k K) bool {
	sm.Lock()
	defer sm.Unlock()
	kv := kv[K, V]{k: k}
	if ok := sm.tr.Has(kv); !ok {
		sm.tr.ReplaceOrInsert(kv)
		return true
	}
	return false
}

// Return true if K was inserted
func (sm *SortedMap[K, V]) Insert(k K, v V) bool {
	sm.Lock()
	defer sm.Unlock()

	kv := kv[K, V]{k: k, v: v, present: true}
	if e, ok := sm.tr.Get(kv); !ok {
		sm.tr.ReplaceOrInsert(kv)
		return true
	} else if !e.present {
		sm.tr.ReplaceOrInsert(kv)
	}
	return false
}

func (sm *SortedMap[K, V]) Delete(k K) bool {
	sm.Lock()
	defer sm.Unlock()

	if _, ok := sm.tr.Delete(kv[K, V]{k: k}); !ok {
		return false
	}
	return true
}
