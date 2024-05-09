package mntclnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Point struct {
	path   path.Path
	fid    sp.Tfid
	closed bool
}

func (p *Point) String() string {
	return fmt.Sprintf("{%v, %v}", p.path, p.fid)
}

type MntTable struct {
	sync.Mutex
	mounts []*Point
}

func newMntTable() *MntTable {
	mnt := &MntTable{}
	mnt.mounts = make([]*Point, 0)
	return mnt
}

// add path, in order of longest path first. if the path
// already exits, return error
func (mnt *MntTable) add(path path.Path, fid sp.Tfid) *serr.Err {
	mnt.Lock()
	defer mnt.Unlock()

	point := &Point{path: path, fid: fid}
	for i, p := range mnt.mounts {
		if path.Equal(p.path) {
			if p.closed {
				db.DPrintf(db.CRASH, "add %v mount closed %v", path, p.path)
				return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed mount)", p.path))
			}
			return serr.NewErr(serr.TErrExists, fmt.Sprintf("%v (mount)", p.path))
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

func (mnt *MntTable) resolveMnt(path path.Path, allowResolve bool) (sp.Tfid, path.Path, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for _, p := range mnt.mounts {
		ok, left := match(p.path, path)
		//db.DPrintf(db.MOUNT, "resolveMnt: p %v path %v ok %v l %v\n", p.path, path, ok, left)
		if ok {
			if p.closed {
				db.DPrintf(db.CRASH, "resolveMnt %v mount closed %v", path, p.path)
				return sp.NoFid, path, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed mount)", path))
			}
			if len(left) == 0 && !allowResolve {
				continue
			}
			//db.DPrintf(db.MOUNT, "resolveMnt succeeded: p %v path %v l %v", p.path, path, left)
			return p.fid, left, nil
		}
	}
	db.DPrintf(db.MOUNT_ERR, "resolveMnt failed: path %v allowResolve %v mnt %v\n", path, allowResolve, mnt.mounts)
	return sp.NoFid, path, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// Umount matches mnt point that is the longest prefix of path, if exact is
// false, or matches path exact, if exact if true.
func (mnt *MntTable) umount(path path.Path, exact bool) (sp.Tfid, path.Path, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for i, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok {
			if p.closed {
				db.DPrintf(db.CRASH, "umount %v mount closed ", p.path)
				return sp.NoFid, path, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed mount)", p.path))
			}
			if len(left) == 0 {
				mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
				db.DPrintf(db.MOUNT, "umount exact %v %v\n", path, p.fid)
				return p.fid, nil, nil
			}
			if !exact {
				mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
				db.DPrintf(db.MOUNT, "umount prefetch %v left %v %v\n", path, left, p.fid)
				return p.fid, left, nil
			}
		}
	}
	// db.DPrintf(db.ALWAYS, "umount: %v %v\n", path, mnt.mounts)
	return sp.NoFid, nil, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// For testing, mark mount point as closed so that client cannot
// communicate and remount server.
func (mnt *MntTable) disconnect(path path.Path) (sp.Tfid, *serr.Err) {
	for _, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok && len(left) == 0 {
			db.DPrintf(db.CRASH, "disconnect mount %v %v\n", path, p.fid)
			p.closed = true
			return p.fid, nil
		}
	}
	return sp.NoFid, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// Where is path mounted at?  For Disconnect; it ignores closed.
func (mnt *MntTable) mountedAt(path path.Path) path.Path {
	for _, p := range mnt.mounts {
		ok, _ := match(p.path, path)
		if ok {
			return p.path
		}
	}
	return nil
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
