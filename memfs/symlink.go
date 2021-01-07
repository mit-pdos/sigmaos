package memfs

import (
	np "ulambda/ninep"
)

type Symlink struct {
	target string
}

func MakeSym() *Symlink {
	s := Symlink{}
	return &s
}

func (s *Symlink) Len() np.Tlength {
	return np.Tlength(len(s.target))
}
