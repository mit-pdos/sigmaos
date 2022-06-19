package sorteddir

import (
	"sync"

	"github.com/umpc/go-sortedmap"

	np "ulambda/ninep"
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

func (sd *SortedDir) Len() int {
	sd.Lock()
	defer sd.Unlock()

	return sd.dents.Len()
}

func (sd *SortedDir) Insert(name string, st *np.Stat) {
	sd.Lock()
	defer sd.Unlock()

	sd.dents.Insert(name, st)
}

func (sd *SortedDir) Iter(f func(string, *np.Stat) bool) {
	sd.Lock()
	defer sd.Unlock()

	sd.dents.IterFunc(false, func(rec sortedmap.Record) bool {
		b := f(rec.Key.(string), rec.Val.(*np.Stat))
		return b
	})

}
