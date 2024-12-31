package fsux

import (
	"sync"

	"sigmaos/api/fs"
	"sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/refmap"
)

// Objects for which a client has an fid. Several clients may have an
// fid for the same object, therefore we ref count the object
// references.
type ObjTable struct {
	sync.Mutex
	*refmap.RefTable[sp.Tpath, fs.FsObj]
}

func NewObjTable() *ObjTable {
	ot := &ObjTable{}
	ot.RefTable = refmap.NewRefTable[sp.Tpath, fs.FsObj](debug.UX)
	return ot
}

func (ot *ObjTable) GetRef(path sp.Tpath) fs.FsObj {
	ot.Lock()
	defer ot.Unlock()

	if e, ok := ot.RefTable.Lookup(path); ok {
		return e.(fs.FsObj)
	}
	return nil
}

func (ot *ObjTable) AllocRef(path sp.Tpath, o fs.FsObj) fs.FsObj {
	ot.Lock()
	defer ot.Unlock()
	e, _ := ot.RefTable.Insert(path, o)
	return e.(fs.FsObj)
}

func (ot *ObjTable) Clunk(p sp.Tpath) {
	ot.Lock()
	defer ot.Unlock()

	ot.RefTable.Delete(p)
}
