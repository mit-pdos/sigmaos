package srv

import (
	"sigmaos/cache"
	"sync"
)

type shard struct {
	sync.Mutex
	cache cache.Tcache
}

func newShard() *shard {
	return &shard{cache: make(cache.Tcache)}
}

func (s *shard) put(key string, val []byte) error {
	s.Lock()
	defer s.Unlock()
	s.cache[key] = val
	return nil
}

func (s *shard) append(key string, val []byte) error {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.cache[key]; !ok {
		s.cache[key] = make([]byte, 0)
	}
	s.cache[key] = append(s.cache[key], val...)
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

func (s *shard) fill(vals cache.Tcache) bool {
	s.Lock()
	defer s.Unlock()

	for k, v := range vals {
		s.cache[k] = v
	}
	return true
}

func (s *shard) dump() cache.Tcache {
	s.Lock()
	defer s.Unlock()

	m := make(cache.Tcache)
	for k, v := range s.cache {
		m[k] = v
	}
	return m
}
