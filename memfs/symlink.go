package memfs

import (
	np "ulambda/ninep"
)

type Symlink struct {
	target []byte
}

func MakeSym() *Symlink {
	s := Symlink{}
	return &s
}

func (s *Symlink) Len() np.Tlength {
	return np.Tlength(len(s.target))
}

func (s *Symlink) write(d []byte) (np.Tsize, error) {
	s.target = d
	return np.Tsize(len(d)), nil
}

func (s *Symlink) read(offset np.Toffset, n np.Tsize) ([]byte, error) {
	if offset >= np.Toffset(len(s.target)) {
		return nil, nil
	}
	return s.target, nil
}
