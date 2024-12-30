package version

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/refmap"
)

type version struct {
	V sp.TQversion
}

func newVersion() version {
	return version{}
}

func (v *version) String() string {
	return fmt.Sprintf("v %d", v.V)
}

type VersionTable struct {
	sync.Mutex
	*refmap.RefTable[sp.Tpath, version]
}

func NewVersionTable() *VersionTable {
	vt := &VersionTable{}
	vt.RefTable = refmap.NewRefTable[sp.Tpath, version](db.VERSION)
	return vt
}

func (vt *VersionTable) GetVersion(path sp.Tpath) sp.TQversion {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.Lookup(path); ok {
		return e.V
	}
	return 0
}

func (vt *VersionTable) Insert(path sp.Tpath) {
	vt.Lock()
	defer vt.Unlock()
	vt.RefTable.Insert(path, newVersion())
}

func (vt *VersionTable) Delete(p sp.Tpath) (bool, error) {
	vt.Lock()
	defer vt.Unlock()
	return vt.RefTable.Delete(p)
}

func (vt *VersionTable) IncVersion(path sp.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.RefTable.Lookup(path); ok {
		v := e
		v.V += 1
		return
	}
}
