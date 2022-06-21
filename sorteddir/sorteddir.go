package sorteddir

import (
	"sync"

	"github.com/umpc/go-sortedmap"
	// np "ulambda/ninep"
)

type SortedDir struct {
	sync.Mutex
	dents *sortedmap.SortedMap
}

func cmp(a, b interface{}) bool {
	if a == b {
		return true
	}
	return false
}

func MkSortedDir() *SortedDir {
	sd := &SortedDir{}
	sd.dents = sortedmap.New(100, cmp)
	return sd
}

func (sd *SortedDir) String() string {
	s := "["
	sd.dents.IterFunc(false, func(rec sortedmap.Record) bool {
		s += rec.Key.(string) + ", "
		return true
	})
	return s + "]"
}

func (sd *SortedDir) Len() int {
	sd.Lock()
	defer sd.Unlock()

	return sd.dents.Len()
}

func (sd *SortedDir) Lookup(n string) (interface{}, bool) {
	sd.Lock()
	defer sd.Unlock()

	return sd.dents.Get(n)
}

func (sd *SortedDir) Insert(name string, e interface{}) bool {
	sd.Lock()
	defer sd.Unlock()
	return sd.dents.Insert(name, e)
}

func (sd *SortedDir) Delete(name string) {
	sd.Lock()
	defer sd.Unlock()

	sd.dents.Delete(name)
}

func (sd *SortedDir) Iter(f func(string, interface{}) bool) {
	sd.Lock()
	defer sd.Unlock()

	sd.dents.IterFunc(false, func(rec sortedmap.Record) bool {
		b := f(rec.Key.(string), rec.Val)
		return b
	})

}
