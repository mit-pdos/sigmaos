package pathclnt

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Point struct {
	path np.Path
	fid  np.Tfid
}

func (p *Point) String() string {
	return fmt.Sprintf("{%v, %v}", p.path, p.fid)
}

type MntTable struct {
	sync.Mutex
	mounts []*Point
	exited bool
}

func makeMntTable() *MntTable {
	mnt := &MntTable{}
	mnt.mounts = make([]*Point, 0)
	return mnt
}

// add path, in order of longest path first. if the path
// already exits, return error
func (mnt *MntTable) add(path np.Path, fid np.Tfid) *np.Err {
	mnt.Lock()
	defer mnt.Unlock()

	point := &Point{path, fid}
	for i, p := range mnt.mounts {
		if path.Eq(p.path) {
			return np.MkErr(np.TErrExists, fmt.Sprintf("%v (mount)", p.path))
		}
		if len(path) > len(p.path) {
			mnts := append([]*Point{point}, mnt.mounts[i:]...)
			mnt.mounts = append(mnt.mounts[:i], mnts...)
			db.DPrintf("MOUNT", "Mount %v %v\n", fid, path)
			return nil
		}
	}
	db.DPrintf("MOUNT", "Mount %v %v\n", fid, path)
	mnt.mounts = append(mnt.mounts, point)
	return nil
}

// prefix match and return postfix
func match(mp np.Path, path np.Path) (bool, np.Path) {
	for i, s := range mp {
		if i >= len(path) {
			return false, nil
		}
		if s != path[i] {
			return false, path[i:]
		}
	}
	return true, path[len(mp):]
}

func matchexact(mp np.Path, path np.Path) bool {
	if len(mp) != len(path) {
		return false
	}
	for i, s := range mp {
		if s != path[i] {
			return false
		}
	}
	return true
}

func (mnt *MntTable) exit() {
	mnt.Lock()
	defer mnt.Unlock()

	mnt.exited = true
}

func (mnt *MntTable) resolve(path np.Path) (np.Tfid, np.Path, *np.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	if mnt.exited {
		return np.NoFid, path, np.MkErr(np.TErrUnreachable, path)
	}

	for _, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok {
			return p.fid, left, nil
		}
	}
	return np.NoFid, path, np.MkErr(np.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// XXX maybe also umount mount points that have path as a prefix
func (mnt *MntTable) umount(path np.Path) (np.Tfid, *np.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for i, p := range mnt.mounts {
		ok := matchexact(p.path, path)
		if ok {
			mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
			db.DPrintf("MOUNT", "umount %v %v\n", path, p.fid)
			return p.fid, nil
		}
	}
	return np.NoFid, np.MkErr(np.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

func (mnt *MntTable) close() error {
	// Forbid any more (auto)mounting
	mnt.exit()

	// now iterate over mount points and umount them (without
	// holding mnt lock).  XXX do the actually work.

	return nil
}
