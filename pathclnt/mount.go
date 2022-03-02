package pathclnt

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	"ulambda/fidclnt"
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
func (mnt *MntTable) add(path []string, fid np.Tfid) *np.Err {
	mnt.Lock()
	defer mnt.Unlock()

	point := &Point{path, fid}
	for i, p := range mnt.mounts {
		if np.IsPathEq(path, p.path) {
			return np.MkErr(np.TErrExists, fmt.Sprintf("mount %v", p.path))
		}
		if len(path) > len(p.path) {
			mnts := append([]*Point{point}, mnt.mounts[i:]...)
			mnt.mounts = append(mnt.mounts[:i], mnts...)
			db.DLPrintf("MOUNT", "Mount %v %v\n", fid, path)
			return nil
		}
	}
	db.DLPrintf("MOUNT", "Mount %v %v\n", fid, path)
	mnt.mounts = append(mnt.mounts, point)
	return nil
}

// prefix match and return postfix
func match(mp []string, path []string) (bool, []string) {
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
	mnt.Lock()
	defer mnt.Unlock()

	mnt.exited = true
}

// XXX Right now, we return EOF once we've "exited". Perhaps it makes more
// sense to return "unknown mount" or something along those lines.
func (mnt *MntTable) resolve(path []string) (np.Tfid, []string, *np.Err) {
	mnt.Lock()
	defer mnt.Unlock()

	if mnt.exited {
		return np.NoFid, path, np.MkErr(np.TErrEOF, path)
	}

	for _, p := range mnt.mounts {
		ok, left := match(p.path, path)
		if ok {
			return p.fid, left, nil
		}
	}
	return np.NoFid, path, np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount %v", path))
}

// XXX maybe also umount mount points that have path as a prefix
func (mnt *MntTable) umount(fidc *fidclnt.FidClnt, path []string) *np.Err {
	mnt.Lock()
	defer mnt.Unlock()

	for i, p := range mnt.mounts {
		ok := matchexact(p.path, path)
		if ok {
			mnt.mounts = append(mnt.mounts[:i], mnt.mounts[i+1:]...)
			db.DLPrintf("MOUNT", "umount %v %v\n", path, p.fid)
			fidc.Free(p.fid)
			return nil
		}
	}
	return np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount %v", path))
}

func (mnt *MntTable) close() error {
	// Forbid any more (auto)mounting
	mnt.exit()

	// now iterate over mount points and umount them (without
	// holding mnt lock).  XXX do the actually work.

	return nil
}
