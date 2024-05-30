package sorteddir

import (
	"fmt"
	"sort"
	"sync"

	"golang.org/x/exp/constraints"

	"sigmaos/rand"
)

type SortedDir[K constraints.Ordered, V any] struct {
	sync.Mutex
	dents  map[K]V
	sorted []K
	rr     int
}

func NewSortedDir[K constraints.Ordered, V any]() *SortedDir[K, V] {
	sd := &SortedDir[K, V]{}
	sd.dents = make(map[K]V)
	sd.sorted = make([]K, 0)
	return sd
}

func (sd *SortedDir[K, V]) roundrobinIndex() int {
	if len(sd.sorted) == 0 {
		return 0
	}
	return sd.rr % len(sd.sorted)
}

func (sd *SortedDir[K, V]) roundrobinNext() int {
	i := sd.roundrobinIndex()
	sd.rr += 1
	return i
}

func (sd *SortedDir[K, V]) Iter(f func(key K, val V) bool) {
	sd.Lock()
	defer sd.Unlock()

	for _, k := range sd.sorted {
		b := f(k, sd.dents[k])
		if !b {
			return
		}
	}
}

func (sd *SortedDir[K, V]) String() string {
	s := "["
	sd.Iter(func(k K, v V) bool {
		s += fmt.Sprintf("%v , ", k)
		return true
	})
	return s + "]"
}

func (sd *SortedDir[K, V]) Len() int {
	sd.Lock()
	defer sd.Unlock()
	return len(sd.dents)
}

func (sd *SortedDir[K, V]) Lookup(n K) (V, bool) {
	sd.Lock()
	defer sd.Unlock()
	e, ok := sd.dents[n]
	return e, ok
}

func (sd *SortedDir[K, V]) Random() (K, bool) {
	sd.Lock()
	defer sd.Unlock()

	if len(sd.sorted) == 0 {
		var k K
		return k, false
	}
	k := sd.sorted[rand.Int64(int64(len(sd.sorted)))]
	return k, true
}

func (sd *SortedDir[K, V]) RoundRobin() (K, bool) {
	sd.Lock()
	defer sd.Unlock()

	if len(sd.sorted) == 0 {
		var k K
		return k, false
	}
	k := sd.sorted[sd.roundrobinNext()]
	return k, true
}

func (sd *SortedDir[K, V]) Keys(s int) []K {
	return sd.sorted[s:]
}

func (sd *SortedDir[K, V]) insertSort(name K) {
	i := sort.Search(len(sd.sorted), func(i int) bool { return sd.sorted[i] >= name })
	new := make([]K, 1)
	sd.sorted = append(sd.sorted, new...)
	copy(sd.sorted[i+1:], sd.sorted[i:])
	sd.sorted[i] = name
	if i < sd.roundrobinIndex() {
		sd.rr += 1
	}
}

func (sd *SortedDir[K, V]) delSort(name K) {
	i := sort.Search(len(sd.sorted), func(i int) bool { return sd.sorted[i] >= name })
	sd.sorted = append(sd.sorted[:i], sd.sorted[i+1:]...)
	if i < sd.roundrobinIndex() {
		if sd.rr > 0 {
			sd.rr -= 1
		}
	}
}

// Return true if K was inserted
func (sd *SortedDir[K, V]) Insert(name K, e V) bool {
	sd.Lock()
	defer sd.Unlock()
	if _, ok := sd.dents[name]; !ok {
		sd.dents[name] = e
		sd.insertSort(name)
		return true
	}
	return false
}

func (sd *SortedDir[K, V]) Delete(name K) bool {
	sd.Lock()
	defer sd.Unlock()

	if _, ok := sd.dents[name]; !ok {
		return false
	}
	delete(sd.dents, name)
	sd.delSort(name)
	return true
}
