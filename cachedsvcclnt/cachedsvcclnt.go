// The cachedsvclnt package is the client side of a [cachedsvc].  It
// watches the directory of cached servers using [dircache] and sends
// the request to one of them using [cachedclnt].
package cachedsvcclnt

import (
	"hash/fnv"
	"strconv"
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/cache"
	"sigmaos/cacheclnt"
	"sigmaos/cachedsvc"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/dircache"
	"sigmaos/fslib"
	"sigmaos/rpc"
	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

func key2server(key string, nserver int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	server := int(h.Sum32()) % nserver
	return server
}

type CachedSvcClnt struct {
	sync.Mutex
	fsl *fslib.FsLib
	cc  *cacheclnt.CacheClnt
	pn  string
	dd  *dircache.DirCache[struct{}]
}

func NewCachedSvcClnt(fsls []*fslib.FsLib, job string) *CachedSvcClnt {
	csc := &CachedSvcClnt{
		fsl: fsls[0],
		pn:  cache.CACHE,
		cc:  cacheclnt.NewCacheClnt(fsls, job, cachesrv.NSHARD),
	}
	dir := csc.pn + cachedsvc.SRVDIR
	csc.dd = dircache.NewDirCache[struct{}](fsls[0], dir, csc.newEntry, db.CACHEDSVCCLNT, db.CACHEDSVCCLNT)
	return csc
}

func (csc *CachedSvcClnt) newEntry(n string) (struct{}, error) {
	return struct{}{}, nil
}

func (csc *CachedSvcClnt) Server(i int) string {
	return csc.pn + cachedsvc.Server(strconv.Itoa(i))
}

func (csc *CachedSvcClnt) StatsSrvs() ([]*rpc.RPCStatsSnapshot, error) {
	n, err := csc.dd.WaitTimedEntriesN(1)
	if err != nil {
		return nil, err
	}
	stats := make([]*rpc.RPCStatsSnapshot, 0, n)
	for i := 0; i < n; i++ {
		st, err := csc.cc.StatsSrv(csc.Server(i))
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, nil
}

func (csc *CachedSvcClnt) Put(key string, val proto.Message) error {
	return csc.PutTraced(nil, key, val)
}

func (csc *CachedSvcClnt) Get(key string, val proto.Message) error {
	return csc.GetTraced(nil, key, val)
}

func (csc *CachedSvcClnt) Delete(key string) error {
	return csc.DeleteTraced(nil, key)
}

func (csc *CachedSvcClnt) GetTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	n, err := csc.dd.WaitTimedEntriesN(1)
	if err != nil {
		return err
	}
	srv := csc.Server(key2server(key, n))
	return csc.cc.GetTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	n, err := csc.dd.WaitTimedEntriesN(1)
	if err != nil {
		return err
	}
	srv := csc.Server(key2server(key, n))
	return csc.cc.PutTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) DeleteTraced(sctx *tproto.SpanContextConfig, key string) error {
	n, err := csc.dd.Nentry()
	if err != nil {
		return err
	}
	srv := csc.Server(key2server(key, n))
	return csc.cc.DeleteTracedFenced(sctx, srv, key, sp.NullFence())
}

func (csc *CachedSvcClnt) StatsClnt() []map[string]*rpc.MethodStat {
	return csc.StatsClnt()
}

func (csc *CachedSvcClnt) Dump(g int) (map[string]string, error) {
	srv := csc.Server(g)
	return csc.cc.DumpSrv(srv)
}

func (csc *CachedSvcClnt) Close() {
	csc.dd.StopWatching()
}
