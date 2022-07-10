package refmap

import (
	"fmt"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type entry struct {
	n int
	e interface{}
}

func mkEntry(i interface{}) *entry {
	e := &entry{}
	e.n = 1
	e.e = i
	return e
}

func (e *entry) String() string {
	return fmt.Sprintf("{n %d %v}", e.n, e.e)
}

type RefTable struct {
	paths map[np.Tpath]*entry
}

func MkRefTable() *RefTable {
	rf := &RefTable{}
	rf.paths = make(map[np.Tpath]*entry)
	return rf
}

func (rf *RefTable) Lookup(p np.Tpath) (interface{}, bool) {
	if e, ok := rf.paths[p]; ok {
		db.DPrintf("REFMAP", "lookup %v %v\n", p, e)
		return e.e, true
	}
	db.DPrintf("REFMAP", "lookup %v no entry\n", p)
	return nil, false
}

func (rf *RefTable) Insert(p np.Tpath, i interface{}) (interface{}, bool) {
	if e, ok := rf.paths[p]; ok {
		e.n += 1
		db.DPrintf("REFMAP", "insert %v %v\n", p, e)
		return e.e, true
	}
	e := mkEntry(i)
	db.DPrintf("REFMAP", "new insert %v %v\n", p, e)
	rf.paths[p] = e
	return e.e, false
}

func (rf *RefTable) Delete(p np.Tpath) {
	e, ok := rf.paths[p]
	if !ok {
		db.DFatalf("delete %v\n", p)
	}
	e.n -= 1
	if e.n <= 0 {
		db.DPrintf("REFMAP", "delete %v\n", p)
		delete(rf.paths, p)
	}
}
