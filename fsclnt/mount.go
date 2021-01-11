package fsclnt

import (
	"fmt"
	np "ulambda/ninep"
)

type Point struct {
	path []string
	fid  np.Tfid
}

func (p *Point) String() string {
	return fmt.Sprintf("{%v, %v}", p.path, p.fid)
}

type Mount struct {
	mounts []*Point
}

func makeMount() *Mount {
	return &Mount{make([]*Point, 0)}
}

// add path, in order of longest path first
func (mnt *Mount) add(path []string, tid np.Tfid) {
	point := &Point{path, tid}
	for i, p := range mnt.mounts {
		if len(path) > len(p.path) {
			mnts := append([]*Point{point}, mnt.mounts[i:]...)
			mnt.mounts = append(mnt.mounts[:i], mnts...)
			return
		}
	}
	mnt.mounts = append(mnt.mounts, point)
}

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

func (mnt *Mount) resolve(path []string) (np.Tfid, []string) {
	for _, p := range mnt.mounts {
		ok, rest := match(p.path, path)
		if ok {
			return p.fid, rest
		}
	}
	return np.NoFid, path
}
