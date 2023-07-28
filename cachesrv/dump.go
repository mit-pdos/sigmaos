package cachesrv

import (
	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/cache/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
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

func (s *CacheSrv) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	cs := &cacheSession{mfs.MakeDevInode(), s.shards, sid}
	db.DPrintf(db.CACHESRV, "mkSession %v %p\n", cs.shards, cs)
	return cs, nil
}

// XXX incremental read
func (cs *cacheSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf(db.CACHESRV, "Dump cache %p %v\n", cs, cs.shards)
	m := make(map[string][]byte)
	for i, _ := range cs.shards {
		cs.shards[i].s.Lock()
		for k, v := range cs.shards[i].s.cache {
			m[k] = v
		}
		cs.shards[i].s.Unlock()
	}

	b, err := proto.Marshal(&cacheproto.CacheDump{Vals: m})
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func (cs *cacheSession) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrNotSupported, nil)
}

func (cs *cacheSession) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.CACHESRV, "Close %v\n", cs.sid)
	return nil
}
