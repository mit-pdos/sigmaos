package fsclnt

import (
	"log"
	np "ulambda/ninep"
)

type Point struct {
	path []string
	fid  np.Tfid
}

type Mount struct {
	mounts []*Point
}

func makeMount() *Mount {
	return &Mount{make([]*Point, 0)}
}

func (m *Mount) add(path []string, tid np.Tfid) {
	m.mounts = append(m.mounts, &Point{path, tid})
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

func (m *Mount) resolve(path []string) (np.Tfid, []string) {
	for _, p := range m.mounts {
		log.Printf("mount %v %v\n", p.path, path)
		ok, rest := match(p.path, path)
		log.Printf("ok %v %v\n", ok, rest)
		if ok {
			return p.fid, rest
		}
	}
	return np.NoFid, path
}
