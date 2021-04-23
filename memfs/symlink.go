package memfs

import (
	"fmt"
	"time"

	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

type Symlink struct {
	*Inode
	target []byte
}

func MakeSym(i *Inode) *Symlink {
	s := Symlink{}
	s.Inode = i
	return &s
}

func (s *Symlink) Size() np.Tlength {
	s.Lock()
	defer s.Unlock()
	return np.Tlength(len(s.target))
}

func (s *Symlink) SetParent(p *Dir) {
	s.Lock()
	defer s.Unlock()
	s.parent = p
}

func (s *Symlink) Stat(ctx npo.CtxI) (*np.Stat, error) {
	s.Lock()
	defer s.Unlock()
	st := s.Inode.stat()
	st.Length = np.Tlength(len(s.target))
	return st, nil
}

func (s *Symlink) Write(ctx npo.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	s.Lock()
	defer s.Unlock()

	if v != np.NoV && s.version != v {
		return 0, fmt.Errorf("Version mismatch")
	}
	s.target = data
	s.version += 1
	s.Mtime = time.Now().Unix()
	return np.Tsize(len(data)), nil
}

func (s *Symlink) Read(ctx npo.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	if v != np.NoV && s.version != v {
		return nil, fmt.Errorf("Version mismatch")
	}
	if offset >= np.Toffset(len(s.target)) {
		return nil, nil
	}
	return s.target, nil
}
