package version

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/refmap"
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
	*refmap.RefTable[fcall.Tpath, *version]
}

func MkVersionTable() *VersionTable {
	vt := &VersionTable{}
	vt.RefTable = refmap.MkRefTable[sp.Tpath, *version](db.VERSION)
	return vt
}

func (vt *VersionTable) GetVersion(path fcall.Tpath) sp.TQversion {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.Lookup(path); ok {
		return e.V
	}
	return 0
}

func (vt *VersionTable) Insert(path fcall.Tpath) {
	vt.Lock()
	defer vt.Unlock()
	vt.RefTable.Insert(path, mkVersion)
}

func (vt *VersionTable) Delete(p fcall.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	vt.RefTable.Delete(p)
}

func (vt *VersionTable) IncVersion(path fcall.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.RefTable.Lookup(path); ok {
		v := e
		v.V += 1
		return
	}
}
