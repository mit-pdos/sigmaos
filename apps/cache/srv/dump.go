package srv

import (
	"google.golang.org/protobuf/proto"

	"sigmaos/apps/cache"
	cacheproto "sigmaos/apps/cache/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfs/inode"
	"sigmaos/memfssrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type cacheSession struct {
	*inode.Inode
	shards shardMap
	sid    sessp.Tsession
}

func (s *CacheSrv) newSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.FsObj, *serr.Err) {
	cs := &cacheSession{mfs.NewDevInode(), s.shards, sid}
	db.DPrintf(db.CACHESRV, "newSession %v %p\n", cs.shards, cs)
	return cs, nil
}

func (cs *cacheSession) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := cs.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	st.SetLength(sp.Tlength(len(cs.shards)))
	return st, nil
}

// XXX incremental read
func (cs *cacheSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf(db.CACHESRV, "Dump cache %p %v\n", cs, cs.shards)
	m := make(cache.Tcache)
	for i, _ := range cs.shards {
		cs.shards[i].s.Lock()
		for k, v := range cs.shards[i].s.cache {
			m[k] = v
		}
		cs.shards[i].s.Unlock()
	}

	b, err := proto.Marshal(&cacheproto.ShardData{Vals: m})
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	return b, nil
}

func (cs *cacheSession) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, serr.NewErr(serr.TErrNotSupported, nil)
}

func (cs *cacheSession) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.CACHESRV, "Close %v\n", cs.sid)
	return nil
}
