package refmap

import (
	"fmt"

	db "sigmaos/debug"
)

//
// Map of ref-counted references of type K to objects of type T.  For
// example, lockmap uses this to have one lock (T) per pathname (K),
// and delete that lock when the last thread is done with the lock.
// The caller is responsible for concurrency control.
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
	debug string
	refs  map[K]*entry[T]
}

func MkRefTable[K comparable, T any](debug string) *RefTable[K, T] {
	rf := &RefTable[K, T]{}
	rf.debug = debug
	rf.refs = make(map[K]*entry[T])
	return rf
}

func (rf *RefTable[K, T]) Lookup(k K) (T, bool) {
	var r T
	if e, ok := rf.refs[k]; ok {
		db.DPrintf(rf.debug+"_REFMAP", "lookup %v %v", k, e)
		return e.e, true
	}
	db.DPrintf(rf.debug+"_REFMAP", "lookup %v no entry", k)
	return r, false
}

func (rf *RefTable[K, T]) Insert(k K, mkT func() T) (T, bool) {
	if e, ok := rf.refs[k]; ok {
		e.n += 1
		db.DPrintf(rf.debug+"_REFMAP", "insert %v %v", k, e)
		return e.e, true
	}
	e := mkEntry(mkT())
	db.DPrintf(rf.debug+"_REFMAP", "new insert %v %v", k, e)
	rf.refs[k] = e
	return e.e, false
}

func (rf *RefTable[K, T]) Delete(k K) bool {
	del := false
	e, ok := rf.refs[k]
	if !ok {
		db.DFatalf("delete %v", k)
	}
	e.n -= 1
	if e.n <= 0 {
		db.DPrintf(rf.debug+"_REFMAP", "delete %v -> %v", k, e.e)
		del = true
		delete(rf.refs, k)
	}
	return del
}
