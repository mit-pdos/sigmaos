package sorteddir

import (
	"fmt"
	"sort"
	"sync"

	"golang.org/x/exp/constraints"
)

type SortedDir[K constraints.Ordered, V any] struct {
	sync.Mutex
	dents  map[K]V
	sorted []K
}

func NewSortedDir[K constraints.Ordered, V any]() *SortedDir[K, V] {
	sd := &SortedDir[K, V]{}
	sd.dents = make(map[K]V)
	sd.sorted = make([]K, 0)
	return sd
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

func (sd *SortedDir[K, V]) Slice(s int) []K {
	return sd.sorted[s:]
}

func (sd *SortedDir[K, V]) insertSort(name K) {
	i := sort.Search(len(sd.sorted), func(i int) bool { return sd.sorted[i] >= name })
	new := make([]K, 1)
	sd.sorted = append(sd.sorted, new...)
	copy(sd.sorted[i+1:], sd.sorted[i:])
	sd.sorted[i] = name
}

func (sd *SortedDir[K, V]) delSort(name K) {
	i := sort.Search(len(sd.sorted), func(i int) bool { return sd.sorted[i] >= name })
	sd.sorted = append(sd.sorted[:i], sd.sorted[i+1:]...)
}

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

func (sd *SortedDir[K, V]) Delete(name K) {
	sd.Lock()
	defer sd.Unlock()

	if _, ok := sd.dents[name]; !ok {
		return
	}
	delete(sd.dents, name)
	sd.delSort(name)
}
