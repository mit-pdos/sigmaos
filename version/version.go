package version

import (
	"fmt"
	"sync"

	np "sigmaos/ninep"
	"sigmaos/refmap"
)

type version struct {
	V np.TQversion
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
	*refmap.RefTable[np.Tpath, *version]
}

func MkVersionTable() *VersionTable {
	vt := &VersionTable{}
	vt.RefTable = refmap.MkRefTable[np.Tpath, *version]()
	return vt
}

func (vt *VersionTable) GetVersion(path np.Tpath) np.TQversion {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.Lookup(path); ok {
		return e.V
	}
	return 0
}

func (vt *VersionTable) Insert(path np.Tpath) {
	vt.Lock()
	defer vt.Unlock()
	vt.RefTable.Insert(path, mkVersion)
}

func (vt *VersionTable) Delete(p np.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	vt.RefTable.Delete(p)
}

func (vt *VersionTable) IncVersion(path np.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	if e, ok := vt.RefTable.Lookup(path); ok {
		v := e
		v.V += 1
		return
	}
}
