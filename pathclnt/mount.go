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

func (mnt *MntTable) resolve(path path.Path, allowResolve bool) (sp.Tfid, path.Path, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	if mnt.exited {
		return sp.NoFid, path, serr.MkErr(serr.TErrUnreachable, path)
	}

	for _, p := range mnt.mounts {
		ok, left := match(p.path, path)
		// db.DPrintf(db.MOUNT, "resolve: p %v path %v ok %v l %v\n", p.path, path, ok, left)
		if ok {
			if len(left) == 0 && !allowResolve {
				continue
			}
			return p.fid, left, nil
		}
	}
	return sp.NoFid, path, serr.MkErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// Umount mnt point that is the longest prefix of path, if exact is
// false, or matches path exact, if exact if true.
func (mnt *MntTable) umount(path path.Path, exact bool) (sp.Tfid, path.Path, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for i, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok && len(left) == 0 {
			mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
			db.DPrintf(db.MOUNT, "umount exact %v %v\n", path, p.fid)
			return p.fid, nil, nil
		}
		if ok && !exact {
			mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
			db.DPrintf(db.MOUNT, "umount prefetch %v left %v %v\n", path, left, p.fid)
			return p.fid, left, nil
		}
	}
	db.DPrintf(db.ALWAYS, "umount: %v %v\n", path, mnt.mounts)
	return sp.NoFid, nil, serr.MkErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
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
