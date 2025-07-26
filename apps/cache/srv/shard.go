package srv

import (
	"sigmaos/apps/cache"
	"sync"
)

type shard struct {
	sync.Mutex
	cache     cache.Tcache
	hitCnt    uint64
	oldHitCnt uint64
}

func newShard() *shard {
	return &shard{
		cache:     make(cache.Tcache),
		hitCnt:    0,
		oldHitCnt: 0,
	}
}

func (s *shard) put(key string, val []byte) error {
	s.Lock()
	defer s.Unlock()

	s.hitCnt++
	s.cache[key] = val
	return nil
}

func (s *shard) append(key string, val []byte) error {
	s.Lock()
	defer s.Unlock()

	s.hitCnt++
	if _, ok := s.cache[key]; !ok {
		s.cache[key] = make([]byte, 0)
	}
	s.cache[key] = append(s.cache[key], val...)
	return nil
}

func (s *shard) get(key string) ([]byte, bool) {
	s.Lock()
	defer s.Unlock()

	s.hitCnt++
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

func (s *shard) getHitCnt() uint64 {
	s.Lock()
	defer s.Unlock()

	return s.oldHitCnt
}

func (s *shard) resetHitCnt() uint64 {
	s.Lock()
	defer s.Unlock()

	s.oldHitCnt = s.hitCnt
	s.hitCnt = 0
	return s.oldHitCnt
}
