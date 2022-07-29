package refmap

import (
	"fmt"

	db "ulambda/debug"
)

//
// Map of ref-counted references of type K
//

type entry[T any] struct {
	n int
	e T
}

func mkEntry[T any](i T) *entry[T] {
	e := &entry[T]{}
	e.n = 1
	e.e = i
	return e
}

func (e *entry[T]) String() string {
	return fmt.Sprintf("{n %d %v}", e.n, e.e)
}

type RefTable[K comparable, T any] struct {
	refs map[K]*entry[T]
}

func MkRefTable[K comparable, T any]() *RefTable[K, T] {
	rf := &RefTable[K, T]{}
	rf.refs = make(map[K]*entry[T])
	return rf
}

func (rf *RefTable[K, T]) Lookup(k K) (T, bool) {
	var r T
	if e, ok := rf.refs[k]; ok {
		db.DPrintf("REFMAP", "lookup %v %v\n", k, e)
		return e.e, true
	}
	db.DPrintf("REFMAP", "lookup %v no entry\n", k)
	return r, false
}

func (rf *RefTable[K, T]) Insert(k K, i T) (T, bool) {
	if e, ok := rf.refs[k]; ok {
		e.n += 1
		db.DPrintf("REFMAP", "insert %v %v\n", k, e)
		return e.e, true
	}
	e := mkEntry(i)
	db.DPrintf("REFMAP", "new insert %v %v\n", k, e)
	rf.refs[k] = e
	return e.e, false
}

func (rf *RefTable[K, T]) Delete(k K) bool {
	del := false
	e, ok := rf.refs[k]
	if !ok {
		db.DFatalf("delete %v\n", k)
	}
	e.n -= 1
	if e.n <= 0 {
		db.DPrintf("REFMAP", "delete %v\n", k)
		del = true
		delete(rf.refs, k)
	}
	return del
}
