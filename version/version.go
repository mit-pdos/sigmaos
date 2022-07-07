package version

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

//  XXX use Tpath instead of Path

type VersionTable struct {
	sync.Mutex
	paths map[np.Tpath]*Version
}

func MkVersionTable() *VersionTable {
	vt := &VersionTable{}
	vt.paths = make(map[np.Tpath]*Version)
	return vt
}

func (vt *VersionTable) GetVersion(path np.Tpath) np.TQversion {
	vt.Lock()
	defer vt.Unlock()

	if v, ok := vt.paths[path]; ok {
		return v.V
	}
	return 0
}

func (vt *VersionTable) Add(path np.Tpath) *Version {
	vt.Lock()
	defer vt.Unlock()

	v, ok := vt.paths[path]
	if ok {
		v.n += 1
		db.DPrintf("VT", "insert %v %v\n", path, v)
		return v
	}
	v = mkVersion()
	db.DPrintf("VT", "new insert %v %v\n", path, v)
	vt.paths[path] = v
	return v
}

func (vt *VersionTable) Delete(p np.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	v, ok := vt.paths[p]
	if !ok {
		db.DFatalf("delete %v\n", p)
	}
	v.n -= 1
	if v.n <= 0 {
		db.DPrintf("VT", "delete %v\n", p)
		delete(vt.paths, p)
	}
}

func (vt *VersionTable) IncVersion(path np.Tpath) {
	vt.Lock()
	defer vt.Unlock()

	if v, ok := vt.paths[path]; ok {
		v.V += 1
	}
}

type Version struct {
	sync.Mutex
	n int
	V np.TQversion
}

func mkVersion() *Version {
	v := &Version{}
	v.n = 1
	return v
}

func (v *Version) String() string {
	return fmt.Sprintf("n %d v %d", v.n, v.V)
}
