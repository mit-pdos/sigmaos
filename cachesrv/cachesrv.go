package cachesrv

import (
	"encoding/json"
	"errors"
	"sync"

	"sigmaos/cachesrv/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/sigmap"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/sessdev"
)

const (
	DUMP = "dump"
)

var (
	ErrMiss = errors.New("cache miss")
)

type cache struct {
	sync.Mutex
	cache map[string][]byte
}

type CacheSrv struct {
	c   *cache
	grp string
}

func RunCacheSrv(args []string) error {
	s := &CacheSrv{}
	if len(args) > 2 {
		s.grp = args[2]
	}
	s.c = &cache{}
	s.c.cache = make(map[string][]byte)
	db.DPrintf("CACHESRV", "%v: Run %v\n", proc.GetName(), s.grp)
	pds, err := protdevsrv.MakeProtDevSrv(np.HOTELCACHE+s.grp, s)
	if err != nil {
		return err
	}
	if err := sessdev.MkSessDev(pds.MemFs, DUMP, s.mkSession); err != nil {
		return err
	}
	return pds.RunServer()
}

// XXX support timeout
func (s *CacheSrv) Set(req proto.CacheRequest, rep *proto.CacheResult) error {
	db.DPrintf("CACHESRV", "%v: Set %v\n", proc.GetName(), req)
	s.c.Lock()
	defer s.c.Unlock()
	s.c.cache[req.Key] = req.Value
	return nil
}

func (s *CacheSrv) Get(req proto.CacheRequest, rep *proto.CacheResult) error {
	db.DPrintf("CACHESRV", "%v: Get %v\n", proc.GetName(), req)
	s.c.Lock()
	defer s.c.Unlock()

	b, ok := s.c.cache[req.Key]
	if ok {
		rep.Value = b
		return nil
	}
	return ErrMiss
}

type cacheSession struct {
	*inode.Inode
	c   *cache
	sid fcall.Tsession
}

func (s *CacheSrv) mkSession(mfs *memfssrv.MemFs, sid fcall.Tsession) (fs.Inode, *fcall.Err) {
	cs := &cacheSession{mfs.MakeDevInode(), s.c, sid}
	db.DPrintf("CACHESRV", "mkSession %v %p\n", cs.c, cs)
	return cs, nil
}

// XXX incremental read
func (cs *cacheSession) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *fcall.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf("CACHESRV", "Dump cache %p %v\n", cs, cs.c)
	b, err := json.Marshal(cs.c.cache)
	if err != nil {
		return nil, fcall.MkErrError(err)
	}
	return b, nil
}

func (cs *cacheSession) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *fcall.Err) {
	return 0, fcall.MkErr(fcall.TErrNotSupported, nil)
}

func (cs *cacheSession) Close(ctx fs.CtxI, m np.Tmode) *fcall.Err {
	db.DPrintf("CACHESRV", "%v: Close %v\n", proc.GetName(), cs.sid)
	return nil
}
