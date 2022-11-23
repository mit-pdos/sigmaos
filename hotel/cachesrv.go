package hotel

import (
	"errors"
	"sync"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

var (
	ErrMiss = errors.New("cache miss")
)

type CacheRequest struct {
	Key   string
	Value []byte
}

type CacheResult struct {
	Value []byte
}

type cache struct {
	sync.Mutex
	cache map[string][]byte
}

type CacheSrv struct {
	c *cache
}

func RunCacheSrv(n string) error {
	s := &CacheSrv{}
	s.c = &cache{}
	s.c.cache = make(map[string][]byte)
	pds, err := protdevsrv.MakeProtDevSrv(np.HOTELCACHE, s)
	if err != nil {
		return err
	}
	return pds.RunServer()
}

// XXX support timeout
func (s *CacheSrv) Set(req CacheRequest, rep *CacheResult) error {
	db.DPrintf("HOTELCACHE", "Set %v\n", req)
	s.c.Lock()
	defer s.c.Unlock()
	s.c.cache[req.Key] = req.Value
	return nil
}

func (s *CacheSrv) Get(req CacheRequest, rep *CacheResult) error {
	db.DPrintf("HOTELCACHE", "Get %v\n", req)
	s.c.Lock()
	defer s.c.Unlock()

	b, ok := s.c.cache[req.Key]
	if ok {
		rep.Value = b
		return nil
	}
	return ErrMiss
}
