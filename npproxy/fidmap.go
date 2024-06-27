package npproxy

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

//
// Map to translate from proxy fids to sigma fids
//

type fidMap struct {
	*syncmap.SyncMap[sp.Tfid, sp.Tfid]
}

func newFidMap() *fidMap {
	return &fidMap{
		SyncMap: syncmap.NewSyncMap[sp.Tfid, sp.Tfid](),
	}
}

func (fm *fidMap) mapTo(fid1, fid2 sp.Tfid) {
	ok := fm.Insert(fid1, fid2)
	if !ok {
		db.DFatalf("mapTo %v", fid1)
	}
}

func (fm *fidMap) lookup(fid1 sp.Tfid) (sp.Tfid, bool) {
	return fm.Lookup(fid1)
}

func (fm *fidMap) delete(fid sp.Tfid) {
	fm.Delete(fid)
}
