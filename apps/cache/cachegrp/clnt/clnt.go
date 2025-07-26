// The cachedsvclnt package is the client side of a [cachedsvc].  It
// watches the directory of cached servers using [dircache] and sends
// the request to one of them using [cachedclnt].
package clnt

import (
	"hash/fnv"
	"strconv"
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/apps/cache"
	"sigmaos/apps/cache/cachegrp"
	cacheclnt "sigmaos/apps/cache/clnt"
	cacheproto "sigmaos/apps/cache/proto"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dircache"
	sp "sigmaos/sigmap"
	tproto "sigmaos/util/tracing/proto"
)

func Key2server(key string, nserver int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	server := int(h.Sum32()) % nserver
	return server
}

type CachedSvcClnt struct {
	sync.Mutex
	fsl            *fslib.FsLib
	epcc           *epcacheclnt.EndpointCacheClnt
	useEPCacheClnt bool
	cc             *cacheclnt.CacheClnt
	pn             string
	dd             *dircache.DirCache[struct{}]
}

func NewCachedSvcClnt(fsl *fslib.FsLib, job string) *CachedSvcClnt {
	return NewCachedSvcClntEPCache(fsl, nil, job)
}

func NewCachedSvcClntEPCache(fsl *fslib.FsLib, epcc *epcacheclnt.EndpointCacheClnt, job string) *CachedSvcClnt {
	csc := &CachedSvcClnt{
		fsl:            fsl,
		epcc:           epcc,
		useEPCacheClnt: epcc != nil,
		pn:             cache.CACHE,
		cc:             cacheclnt.NewCacheClnt(fsl, job, cache.NSHARD),
	}
	dir := csc.pn + cachegrp.SRVDIR
	csc.dd = dircache.NewDirCache[struct{}](fsl, dir, csc.newEntry, nil, db.CACHEDSVCCLNT, db.CACHEDSVCCLNT)
	return csc
}

func (csc *CachedSvcClnt) newEntry(n string) (struct{}, error) {
	return struct{}{}, nil
}

func (csc *CachedSvcClnt) Server(i int) string {
	return csc.pn + cachegrp.Server(strconv.Itoa(i))
}

func (csc *CachedSvcClnt) BackupServer(i int) string {
	return csc.pn + cachegrp.BackupServer(strconv.Itoa(i))
}

func (csc *CachedSvcClnt) StatsSrvs() ([]*rpc.RPCStatsSnapshot, error) {
	n, err := csc.dd.WaitEntriesN(1, true)
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

func (csc *CachedSvcClnt) GetEndpoint(i int) (*sp.Tendpoint, error) {
	// Read the endpoint of the endpoint cache server
	srvEPB, err := csc.fsl.GetFile(csc.Server(i))
	if err != nil {
		return nil, err
	}
	srvEP, err := sp.NewEndpointFromBytes(srvEPB)
	if err != nil {
		return nil, err
	}
	return srvEP, nil
}

func (csc *CachedSvcClnt) GetEndpoints() (map[string]*sp.Tendpoint, error) {
	n, err := csc.dd.WaitEntriesN(1, true)
	if err != nil {
		return nil, err
	}
	eps := make(map[string]*sp.Tendpoint)
	for i := 0; i < n; i++ {
		ep, err := csc.GetEndpoint(i)
		if err != nil {
			return nil, err
		}
		eps[csc.Server(i)] = ep
	}
	return eps, nil
}

func NewMultiGetReqs(keys []string, nserver int, nshard uint32) map[int]*cacheproto.CacheMultiGetReq {
	reqs := make(map[int]*cacheproto.CacheMultiGetReq)
	for _, key := range keys {
		server := Key2server(key, nserver)
		req, ok := reqs[server]
		if !ok {
			req = &cacheproto.CacheMultiGetReq{
				Fence: sp.NullFence().FenceProto(),
			}
			reqs[server] = req
		}
		req.Gets = append(req.Gets, &cacheproto.CacheGetDescriptor{
			Key:   key,
			Shard: cacheclnt.Key2shard(key, nshard),
		})
	}
	return reqs
}

// XXX Fences?
// Do we need a non-null fence?
func (cs *CachedSvcClnt) NewDumpReq(shard cache.Tshard) *cacheproto.ShardReq {
	req := &cacheproto.ShardReq{
		Shard: uint32(shard),
		Fence: sp.NullFence().FenceProto(),
	}
	return req
}

func (csc *CachedSvcClnt) Key2shard(key string) uint32 {
	return csc.cc.Key2shard(key)
}

func (csc *CachedSvcClnt) Put(key string, val proto.Message) error {
	return csc.PutTraced(nil, key, val)
}

func (csc *CachedSvcClnt) PutBytes(key string, b []byte) error {
	return csc.PutBytesTraced(nil, key, b)
}

func (csc *CachedSvcClnt) Get(key string, val proto.Message) error {
	return csc.getTraced(nil, key, val, false)
}

func (csc *CachedSvcClnt) BackupGet(key string, val proto.Message) error {
	return csc.getTraced(nil, key, val, true)
}

func (csc *CachedSvcClnt) Delete(key string) error {
	return csc.DeleteTraced(nil, key)
}

func (csc *CachedSvcClnt) GetHotShards(srv int, topN uint32) ([]cache.Tshard, []uint64, error) {
	return csc.cc.GetHotShards(csc.Server(srv), topN)
}

func (csc *CachedSvcClnt) GetTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	return csc.getTraced(sctx, key, val, false)
}

func (csc *CachedSvcClnt) getTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message, backup bool) error {
	n, err := csc.dd.WaitEntriesN(1, true)
	if err != nil {
		return err
	}
	var srv string
	if backup {
		srv = csc.BackupServer(Key2server(key, n))
	} else {
		srv = csc.Server(Key2server(key, n))
	}
	return csc.cc.GetTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) PutBytesTraced(sctx *tproto.SpanContextConfig, key string, b []byte) error {
	n, err := csc.dd.WaitEntriesN(1, true)
	if err != nil {
		return err
	}
	srv := csc.Server(Key2server(key, n))
	return csc.cc.PutBytesTracedFenced(sctx, srv, key, b, sp.NullFence())
}

func (csc *CachedSvcClnt) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	n, err := csc.dd.WaitEntriesN(1, true)
	if err != nil {
		return err
	}
	srv := csc.Server(Key2server(key, n))
	return csc.cc.PutTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) DeleteTraced(sctx *tproto.SpanContextConfig, key string) error {
	n, err := csc.dd.Nentry()
	if err != nil {
		return err
	}
	srv := csc.Server(Key2server(key, n))
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
