package fsobjsrv

import (
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	np "ulambda/ninep"
)

type fidTable struct {
	sync.Mutex
	fids map[np.Tfid]*fid.Fid
}

func makeFidTable() *fidTable {
	ft := &fidTable{}
	ft.fids = make(map[np.Tfid]*fid.Fid)
	return ft
}

func (ft *fidTable) Lookup(fid np.Tfid) (*fid.Fid, bool) {
	ft.Lock()
	defer ft.Unlock()

	f, ok := ft.fids[fid]
	return f, ok
}

func (ft *fidTable) Add(fid np.Tfid, f *fid.Fid) {
	ft.Lock()
	defer ft.Unlock()

	ft.fids[fid] = f
}

func (ft *fidTable) Del(fid np.Tfid) (fs.FsObj, bool) {
	ft.Lock()
	defer ft.Unlock()

	o := ft.fids[fid].ObjU()
	delete(ft.fids, fid)
	return o, true
}

func (ft *fidTable) ClunkOpen() {
	ft.Lock()
	defer ft.Unlock()

	log.Printf("%v: Clunkopen: %v\n", db.GetName(), ft.fids)
	for fid, f := range ft.fids {
		o := ft.fids[fid].ObjU()
		if f.IsOpen() { // has the fid been opened?
			log.Printf("Clunkopen: %v close: %v\n", f.Ctx().Uname(), f.Path())
			o.Close(f.Ctx(), f.Mode())
		}
	}
}
