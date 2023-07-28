package cachesrv

import (
	"sync"
	// sp "sigmaos/sigmap"
)

type shard struct {
	sync.Mutex
	cache map[string][]byte
}

// XXX support timeout
func (s *shard) put(key string, val []byte) error {
	s.Lock()
	defer s.Unlock()
	s.cache[key] = val
	return nil
}

func (s *shard) get(key string) ([]byte, bool) {
	s.Lock()
	defer s.Unlock()

	v, ok := s.cache[key]
	return v, ok
}

func (s *shard) delete(key string) bool {
	s.Lock()
	defer s.Unlock()

	_, ok := s.cache[key]
	if ok {
		delete(s.cache, key)
		return true
	}
	return false
}
