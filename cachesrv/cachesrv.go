package cachesrv

import (
	"encoding/json"
	"errors"
	"sync"

	"sigmaos/cachesrv/proto"
	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/sessdev"
	sp "sigmaos/sigmap"
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
	c    *cache
	shrd string
}

func RunCacheSrv(args []string) error {
	s := &CacheSrv{}
	if len(args) > 2 {
		s.shrd = args[2]
	}
	s.c = &cache{}
	s.c.cache = make(map[string][]byte)
	db.DPrintf(db.CACHESRV, "%v: Run %v\n", proc.GetName(), s.shrd)
	pds, err := protdevsrv.MakeProtDevSrv(sp.CACHE+s.shrd, s)
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
	db.DPrintf(db.CACHESRV, "%v: Set %v\n", proc.GetName(), req)
	s.c.Lock()
	defer s.c.Unlock()
	s.c.cache[req.Key] = req.Value
	return nil
}

func (s *CacheSrv) Get(req proto.CacheRequest, rep *proto.CacheResult) error {
	db.DPrintf(db.CACHESRV, "%v: Get %v\n", proc.GetName(), req)
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
	sid sessp.Tsession
}

func (s *CacheSrv) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	cs := &cacheSession{mfs.MakeDevInode(), s.c, sid}
	db.DPrintf(db.CACHESRV, "mkSession %v %p\n", cs.c, cs)
	return cs, nil
}

// XXX incremental read
func (cs *cacheSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf(db.CACHESRV, "Dump cache %p %v\n", cs, cs.c)
	b, err := json.Marshal(cs.c.cache)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func (cs *cacheSession) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrNotSupported, nil)
}

func (cs *cacheSession) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.CACHESRV, "%v: Close %v\n", proc.GetName(), cs.sid)
	return nil
}
