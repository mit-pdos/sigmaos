// Package version maintains a version number for a
// Tpath. [spproto/srv] uses it to detect if a directory has changed
// since opening it for reading and setting a watch on it.
package version

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/refmap"
)

const (
	N = 1000
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
	vt.RefTable = refmap.NewRefTable[sp.Tpath, version](N, db.VERSION)
	return vt
}

func (vt *VersionTable) Len() (int, int) {
	vt.Lock()
	defer vt.Unlock()
	return vt.RefTable.Len()
}

func (vt *VersionTable) GetVersion(path sp.Tpath) sp.TQversion {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.Lookup(path); ok {
		return e.V
	}
	return 0
}

func (vt *VersionTable) Insert(path sp.Tpath) (sp.TQversion, bool) {
	vt.Lock()
	defer vt.Unlock()
	e, ok := vt.RefTable.Insert(path, newVersion())
	return e.V, ok
}

func (vt *VersionTable) Delete(p sp.Tpath) (bool, error) {
	vt.Lock()
	defer vt.Unlock()
	return vt.RefTable.Delete(p)
}

func (vt *VersionTable) IncVersion(path sp.Tpath) (sp.TQversion, bool) {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.RefTable.Lookup(path); ok {
		e.V += 1
		return e.V, true
	}
	return 0, false
}
