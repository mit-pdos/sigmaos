package sorteddir

import (
	"sort"
	"sync"
)

type SortedDir struct {
	sync.Mutex
	dents  map[string]interface{}
	sorted []string
}

func MkSortedDir() *SortedDir {
	sd := &SortedDir{}
	sd.dents = make(map[string]interface{})
	sd.sorted = make([]string, 0)
	return sd
}

func (sd *SortedDir) Iter(f func(string, interface{}) bool) {
	sd.Lock()
	defer sd.Unlock()

	for _, k := range sd.sorted {
		b := f(k, sd.dents[k])
		if !b {
			return
		}
	}
}

func (sd *SortedDir) String() string {
	s := "["
	sd.Iter(func(k string, v interface{}) bool {
		s += k + ", "
		return true
	})
	return s + "]"
}

func (sd *SortedDir) Len() int {
	sd.Lock()
	defer sd.Unlock()
	return len(sd.dents)
}

func (sd *SortedDir) Lookup(n string) (interface{}, bool) {
	sd.Lock()
	defer sd.Unlock()
	e, ok := sd.dents[n]
	return e, ok
}

func (sd *SortedDir) insertSort(name string) {
	i := sort.SearchStrings(sd.sorted, name)
	sd.sorted = append(sd.sorted, "")
	copy(sd.sorted[i+1:], sd.sorted[i:])
	sd.sorted[i] = name
}

func (sd *SortedDir) delSort(name string) {
	i := sort.SearchStrings(sd.sorted, name)
	sd.sorted = append(sd.sorted[:i], sd.sorted[i+1:]...)
}

func (sd *SortedDir) Insert(name string, e interface{}) bool {
	sd.Lock()
	defer sd.Unlock()
	if _, ok := sd.dents[name]; !ok {
		sd.dents[name] = e
		sd.insertSort(name)
		return true
	}
	return false
}

func (sd *SortedDir) Delete(name string) {
	sd.Lock()
	defer sd.Unlock()

	if _, ok := sd.dents[name]; !ok {
		return
	}
	delete(sd.dents, name)
	sd.delSort(name)
}
