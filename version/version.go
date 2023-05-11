package version

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/refmap"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type version struct {
	V sp.TQversion
}

func mkVersion() *version {
	v := &version{}
	v.V = 0
	return v
}

func (v *version) String() string {
	return fmt.Sprintf("v %d", v.V)
}

type VersionTable struct {
	sync.Mutex
	*refmap.RefTable[sessp.Tpath, *version]
}

func MkVersionTable() *VersionTable {
	vt := &VersionTable{}
	vt.RefTable = refmap.MkRefTable[sessp.Tpath, *version](db.VERSION)
	return vt
}

func (vt *VersionTable) GetVersion(path sessp.Tpath) sp.TQversion {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.Lookup(path); ok {
		return e.V
	}
	return 0
}

func (vt *VersionTable) Insert(path sessp.Tpath) {
	vt.Lock()
	defer vt.Unlock()
	vt.RefTable.Insert(path, mkVersion)
}

func (vt *VersionTable) Delete(p sessp.Tpath) {
	vt.Lock()
	defer vt.Unlock()
	vt.RefTable.Delete(p)
}

func (vt *VersionTable) IncVersion(path sessp.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.RefTable.Lookup(path); ok {
		v := e
		v.V += 1
		return
	}
}
