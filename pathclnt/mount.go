package pathclnt

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Point struct {
	path []string
	fid  np.Tfid
}

func (p *Point) String() string {
	return fmt.Sprintf("{%v, %v}", p.path, p.fid)
}

type MntTable struct {
	mu     sync.Mutex
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
func (mnt *MntTable) add(path []string, fid np.Tfid) *np.Err {
	mnt.mu.Lock()
	defer mnt.mu.Unlock()

	point := &Point{path, fid}
	for i, p := range mnt.mounts {
		if np.IsPathEq(path, p.path) {
			return np.MkErr(np.TErrExists, fmt.Sprintf("mount %v", p.path))
		}
		if len(path) > len(p.path) {
			mnts := append([]*Point{point}, mnt.mounts[i:]...)
			mnt.mounts = append(mnt.mounts[:i], mnts...)
			return nil
		}
	}
	mnt.mounts = append(mnt.mounts, point)
	return nil
}

// prefix match
func match(mp []string, path []string) (bool, []string) {
	rest := path
	for _, s := range mp {
		if len(rest) == 0 {
			return false, rest
		}
		if s == rest[0] {
			rest = rest[1:]
		} else {
			return false, rest
		}
	}
	return true, rest
}

func matchexact(mp []string, path []string) bool {
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
	mnt.mu.Lock()
	defer mnt.mu.Unlock()

	mnt.exited = true
}

// XXX Right now, we return EOF once we've "exited". Perhaps it makes more
// sense to return "unknown mount" or something along those lines.
func (mnt *MntTable) hasExited() bool {
	mnt.mu.Lock()
	defer mnt.mu.Unlock()

	return mnt.exited
}

func (mnt *MntTable) resolve(path []string) (np.Tfid, []string) {
	mnt.mu.Lock()
	defer mnt.mu.Unlock()

	if mnt.exited {
		db.DLPrintf("FSCLNT", "resolve %v %v failed: mount exited \n", mnt.mounts, path)
		return np.NoFid, path
	}

	for _, p := range mnt.mounts {
		ok, rest := match(p.path, path)
		if ok {
			return p.fid, rest
		}
	}
	return np.NoFid, path
}

func (mnt *MntTable) umount(path []string) (np.Tfid, *np.Err) {
	mnt.mu.Lock()
	defer mnt.mu.Unlock()

	db.DLPrintf("FSCLNT", "umount %v\n", path)
	for i, p := range mnt.mounts {
		ok := matchexact(p.path, path)
		if ok {
			mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
			db.DLPrintf("FSCLNT", "umount -> %v\n", mnt.mounts)
			return p.fid, nil
		}
	}
	return np.NoFid, np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount %v\n", path))
}
