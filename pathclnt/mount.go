package pathclnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Point struct {
	path path.Path
	fid  sp.Tfid
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
func (mnt *MntTable) add(path path.Path, fid sp.Tfid) *serr.Err {
	mnt.Lock()
	defer mnt.Unlock()

	point := &Point{path, fid}
	for i, p := range mnt.mounts {
		if path.Eq(p.path) {
			return serr.MkErr(serr.TErrExists, fmt.Sprintf("%v (mount)", p.path))
		}
		if len(path) > len(p.path) {
			mnts := append([]*Point{point}, mnt.mounts[i:]...)
			mnt.mounts = append(mnt.mounts[:i], mnts...)
			db.DPrintf(db.MOUNT, "Mount %v %v\n", fid, path)
			return nil
		}
	}
	db.DPrintf(db.MOUNT, "Mount %v '%v'\n", fid, path)
	mnt.mounts = append(mnt.mounts, point)
	return nil
}

// prefix match and return postfix
func match(mp path.Path, path path.Path) (bool, path.Path) {
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

func matchexact(mp path.Path, path path.Path) bool {
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

func (mnt *MntTable) resolve(path path.Path, resolve bool) (sp.Tfid, path.Path, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	if mnt.exited {
		return sp.NoFid, path, serr.MkErr(serr.TErrUnreachable, path)
	}

	for _, p := range mnt.mounts {
		// db.DPrintf(db.MOUNT, "mnt %v path %v\n", p.path, path)
		ok, left := match(p.path, path)
		if ok {
			if len(left) == 0 && !resolve {
				continue
			}
			return p.fid, left, nil
		}
	}
	return sp.NoFid, path, serr.MkErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// XXX maybe also umount mount points that have path as a prefix
func (mnt *MntTable) umount(path path.Path) (sp.Tfid, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for i, p := range mnt.mounts {
		ok := matchexact(p.path, path)
		if ok {
			mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
			db.DPrintf(db.MOUNT, "umount %v %v\n", path, p.fid)
			return p.fid, nil
		}
	}
	return sp.NoFid, serr.MkErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

func (mnt *MntTable) mountedPaths() []string {
	mnt.Lock()
	defer mnt.Unlock()

	paths := make([]string, 0, len(mnt.mounts))
	for _, p := range mnt.mounts {
		paths = append(paths, p.path.String())
	}
	return paths
}
