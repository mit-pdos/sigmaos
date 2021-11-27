package memfs

import (
	"fmt"
	"sync"
	"time"

	"ulambda/fs"
	np "ulambda/ninep"
)

type Symlink struct {
	fs.FsObj
	mu     sync.Mutex
	target []byte
}

func MakeSym(i fs.FsObj) *Symlink {
	s := Symlink{}
	s.FsObj = i
	return &s
}

func (s *Symlink) Size() np.Tlength {
	s.mu.Lock()
	defer s.mu.Unlock()
	return np.Tlength(len(s.target))
}

func (s *Symlink) Stat(ctx fs.CtxI) (*np.Stat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = np.Tlength(len(s.target))
	return st, nil
}

func (s *Symlink) Write(ctx fs.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !np.VEq(v, s.Version()) {
		return 0, fmt.Errorf("Version mismatch")
	}
	s.target = data
	s.VersionInc()
	s.SetMtime(time.Now().Unix())
	return np.Tsize(len(data)), nil
}

func (s *Symlink) Read(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !np.VEq(v, s.Version()) {
		return nil, fmt.Errorf("Version mismatch")
	}
	if offset >= np.Toffset(len(s.target)) {
		return nil, nil
	}
	return s.target, nil
}
