package fs

import (
	np "ulambda/ninep"
)

type Symlink struct {
	target string
}

func MakeSym(target string) *Symlink {
	s := Symlink{target}
	return &s
}

func (s *Symlink) Len() np.Tlength {
	return np.Tlength(len(s.target))
}
