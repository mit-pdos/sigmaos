// The refmap package maintains a map of ref-counted references of
// type K to objects of type T.  For example, [lockmap] uses this to
// have one lock (T) per pathname (K), and delete that lock when the
// last thread is done with the lock.  The caller is responsible for
// concurrency control.
package refmap

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/util/freelist"
)

type entry[T any] struct {
	n int
	v T
}

func (e *entry[T]) String() string {
	return fmt.Sprintf("{n %d %v}", e.n, e.v)
}

type RefTable[K comparable, T any] struct {
	debug db.Tselector
	refs  map[K]*entry[T]
	fl    *freelist.FreeList[entry[T]]
}

func NewRefTable[K comparable, T any](sz int, debug db.Tselector) *RefTable[K, T] {
	rf := &RefTable[K, T]{
		debug: debug + db.REFMAP_SUFFIX,
		refs:  make(map[K]*entry[T]),
		fl:    freelist.NewFreeList[entry[T]](sz),
	}
	return rf
}

func (rf *RefTable[K, T]) newEntry(v T) *entry[T] {
	e := rf.fl.New()
	e.n = 1
	e.v = v
	return e
}

func (rf *RefTable[K, T]) Len() (int, int) {
	return len(rf.refs), rf.fl.Len()
}

func (rf *RefTable[K, T]) Lookup(k K) (*T, bool) {
	if e, ok := rf.refs[k]; ok {
		db.DPrintf(rf.debug, "lookup %v %v", k, e)
		return &e.v, true
	}
	db.DPrintf(rf.debug, "lookup %v no entry", k)
	return nil, false
}

func (rf *RefTable[K, T]) Insert(k K, v T) (*T, bool) {
	if e, ok := rf.refs[k]; ok {
		e.n += 1
		db.DPrintf(rf.debug, "insert %v %v", k, e)
		return &e.v, true
	}
	e := rf.newEntry(v)
	db.DPrintf(rf.debug, "new insert %v %v", k, e)
	rf.refs[k] = e
	return &e.v, false
}

func (rf *RefTable[K, T]) Delete(k K) (bool, error) {
	del := false
	e, ok := rf.refs[k]
	if !ok {
		db.DPrintf(db.ERROR, "delete %v %v", rf.debug, k)
		return false, fmt.Errorf("Delete: %v not present\n", k)
	}
	e.n -= 1
	if e.n <= 0 {
		// db.DPrintf(rf.debug, "delete %v -> %v", k, e.v)
		del = true
		delete(rf.refs, k)
		rf.fl.Free(e)
	}
	return del, nil
}
