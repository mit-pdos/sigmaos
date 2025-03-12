package fsclnt

import (
	"strings"

	db "sigmaos/debug"
	"sigmaos/path"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

//
// Keep track of registered fences for a pathname so that caller of
// pathclnt doesn't have to provide a fence as argument on each call.
//

type FenceTable struct {
	fencedDirs *syncmap.SyncMap[sp.Tsigmapath, sp.Tfence]
}

func newFenceTable() *FenceTable {
	ft := &FenceTable{
		fencedDirs: syncmap.NewSyncMap[sp.Tsigmapath, sp.Tfence](),
	}
	return ft
}

// If already exist, just update
func (ft *FenceTable) insert(pn sp.Tsigmapath, f sp.Tfence) error {
	path := path.Split(pn) // cleans up pn
	db.DPrintf(db.FSCLNT, "Insert fence %v %v\n", path, f)
	ft.fencedDirs.InsertBlind(path.String(), f)
	return nil
}

func (ft *FenceTable) lookup(pn sp.Tsigmapath) *sp.Tfence {
	f := sp.NullFence()
	ft.fencedDirs.Iter(func(pni sp.Tsigmapath, f0 sp.Tfence) bool {
		db.DPrintf(db.FSCLNT, "Lookup fence %v %v\n", pn, f0)
		if strings.HasPrefix(pn, pni) {
			f = &f0
			return false
		}
		return true
	})
	db.DPrintf(db.FSCLNT, "Lookup fence %v: fence %v", pn, f)
	return f
}
