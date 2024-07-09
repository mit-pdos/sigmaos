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
	sync.Mutex
	path   path.Tpathname
	fid    sp.Tfid
	closed bool
}

func newPoint(path path.Tpathname, fid sp.Tfid) *Point {
	return &Point{path: path, fid: fid}
}

func (p *Point) String() string {
	return fmt.Sprintf("{%v, %v}", p.path, p.fid)
}

func (p *Point) isAttached() bool {
	return p.fid != sp.NoFid
}

func (p *Point) getFid() (sp.Tfid, bool) {
	p.Lock()
	defer p.Unlock()

	return p.fid, p.isAttached()
}

// For testing, mark mount point as closed so that client cannot
// communicate and remount server.
func (p *Point) disconnect() {
	p.Lock()
	defer p.Unlock()

	db.DPrintf(db.CRASH, "disconnect %v", p)
	p.closed = true
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

func (mnt *MntTable) String() string {
	return fmt.Sprintf("{mounts %v}", mnt.mounts)
}

// If not present, add a place-holder point for path, in order of
// longest path first.  The caller is responsible for the attach.
func (mnt *MntTable) lookupAlloc(path path.Tpathname, fid sp.Tfid) (*Point, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	point := newPoint(path, fid)
	for i, p := range mnt.mounts {
		if path.Equal(p.path) {
			if p.closed {
				db.DPrintf(db.CRASH, "lookupAlloc %v mount closed %v", path, p.path)
				return nil, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed mount)", p.path))
			}
			return p, nil
		}
		if len(path) > len(p.path) {
			mnts := append([]*Point{point}, mnt.mounts[i:]...)
			mnt.mounts = append(mnt.mounts[:i], mnts...)
			db.DPrintf(db.MOUNT, "lookupAlloc %v '%v'\n", point, path)
			return point, nil
		}
	}
	db.DPrintf(db.MOUNT, "lookupAlloc %v '%v'\n", point, path)
	mnt.mounts = append(mnt.mounts, point)
	return point, nil
}

// Match prefix and return postfix
func match(mp path.Tpathname, path path.Tpathname) (bool, path.Tpathname) {
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

func (mnt *MntTable) resolveMnt(path path.Tpathname, allowResolve bool) (*Point, path.Tpathname, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for _, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok {
			if p.closed {
				db.DPrintf(db.CRASH, "resolveMnt %v mount closed %v", path, p.path)
				return nil, path, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed mount)", path))
			}
			if len(left) == 0 && !allowResolve {
				continue
			}
			return p, left, nil
		}
	}
	return nil, path, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// Umount matches mnt point that is the longest prefix of path, if exact is
// false, or matches path exact, if exact if true.
func (mnt *MntTable) umount(path path.Tpathname, exact bool) (*Point, *serr.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	for i, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok {
			if p.closed {
				db.DPrintf(db.CRASH, "umount %v mount closed ", p.path)
				return nil, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed mount)", p.path))
			}
			if len(left) == 0 {
				mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
				db.DPrintf(db.MOUNT, "umount exact %v %v\n", path, p)
				return p, nil
			}
			if !exact {
				mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
				db.DPrintf(db.MOUNT, "umount prefetch %v left %v %v\n", path, left, p.fid)
				return p, nil
			}
		}
	}
	// db.DPrintf(db.ALWAYS, "umount: %v %v\n", path, mnt.mounts)
	return nil, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (no mount)", path))
}

// Where is path mounted at?
func (mnt *MntTable) isMountedAt(path path.Tpathname) (*Point, bool) {
	mnt.Lock()
	defer mnt.Unlock()
	for _, p := range mnt.mounts {
		if ok, _ := match(p.path, path); ok {
			return p, true
		}
	}
	return nil, false
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
