// The cachedsvclnt package is the client side of a [cachedsvc].  It
// watches the directory of cached servers using [dircache] and sends
// the request to one of them using [cachedclnt].
package clnt

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"sigmaos/apps/cache"
	"sigmaos/apps/cache/cachegrp"
	cacheclnt "sigmaos/apps/cache/clnt"
	cacheproto "sigmaos/apps/cache/proto"
	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	"sigmaos/rpc"
	rpcncclnt "sigmaos/rpc/clnt/netconn"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dircache"
	sp "sigmaos/sigmap"
	tproto "sigmaos/util/tracing/proto"
)

func Key2server(key string, nserver int) int {
	shard := cacheclnt.Key2shard(key, cache.NSHARD)
	return int(shard % uint32(nserver))
}

type CachedSvcClnt struct {
	sync.Mutex
	fsl            *fslib.FsLib
	epcc           *epcacheclnt.EndpointCacheClnt
	lastEPV        epcache.Tversion
	useEPCacheClnt bool
	cc             *cacheclnt.CacheClnt
	pn             string
	dir            string
	dd             *dircache.DirCache[struct{}]
	eps            []*sp.Tendpoint
	nsrv           int
	done           bool
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
		cc:             cacheclnt.NewCacheClnt(fsl, job, cache.NSHARD, false),
		eps:            make([]*sp.Tendpoint, 0, 1),
		lastEPV:        epcache.NO_VERSION,
	}
	csc.dir = filepath.Join(csc.pn, cachegrp.SRVDIR)
	csc.dd = dircache.NewDirCache[struct{}](fsl, csc.dir, csc.newEntry, nil, db.CACHEDSVCCLNT, db.CACHEDSVCCLNT)
	if csc.useEPCacheClnt {
		go csc.monitorServers()
	}
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

func (csc *CachedSvcClnt) monitorServers() {
	for !csc.done {
		// Get EPs
		instances, v, err := csc.epcc.GetEndpoints(csc.dir, csc.lastEPV)
		if err != nil {
			db.DPrintf(db.CACHEDSVCCLNT, "Err GetEndpoints: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		db.DPrintf(db.CACHEDSVCCLNT, "Detected new number of instances: %v", len(instances))
		// Create clients for each of the new instances, if any of them are CPP cache servers
		for i := csc.nsrv; i < len(instances); i++ {
			istr := strconv.Itoa(i)
			for _, is := range instances {
				if is.ID == istr {
					ep := sp.NewEndpointFromProto(is.EndpointProto)
					// Append the new endpoint
					csc.eps = append(csc.eps, ep)
					if sp.TTendpoint(is.EndpointProto.Type) == sp.CPP_EP {
						rpcc, err := rpcncclnt.NewTCPRPCClnt(is.ID, ep, 0)
						if err != nil {
							db.DPrintf(db.ERROR, "Err NewRPCClnt cacheclnt: %v", err)
						} else {
							db.DPrintf(db.CACHEDSVCCLNT, "Create new cacheclnt for cache %v", is.ID)
							csc.cc.ClntCache.Put(csc.Server(i), rpcc)
						}
					}
				}
			}
		}
		csc.nsrv = len(instances)
		// If the endpoint for a server changed, create a new client for it.
		for _, is := range instances {
			id, err := strconv.Atoi(is.ID)
			if err != nil {
				db.DPrintf(db.ERROR, "Err NewRPCClnt cacheclnt cache ID: %v", err)
			}
			// EP has been removed, so remove its cache clnt
			if len(csc.eps[id].Addr) == 0 {
				db.DPrintf(db.CACHEDSVCCLNT, "EP removed for cache %v", is.ID)
				csc.cc.ClntCache.Delete(csc.Server(id))
				continue
			}
			// EP has changed, so create a new cache clnt
			if csc.eps[id].Addr[0].IPStr != is.EndpointProto.Addr[0].IPStr ||
				csc.eps[id].Addr[0].PortInt != is.EndpointProto.Addr[0].PortInt ||
				csc.eps[id].Type != is.EndpointProto.Type {
				db.DPrintf(db.CACHEDSVCCLNT, "EP has changed for cache %v", is.ID)
				ep := sp.NewEndpointFromProto(is.EndpointProto)
				csc.eps[id] = ep
				rpcc, err := rpcncclnt.NewTCPRPCClnt(is.ID, ep, 0)
				if err != nil {
					db.DPrintf(db.ERROR, "Err NewRPCClnt cacheclnt: %v", err)
				} else {
					db.DPrintf(db.CACHEDSVCCLNT, "Create new cacheclnt for cache %v", is.ID)
					csc.cc.ClntCache.Put(csc.Server(id), rpcc)
				}
			}
		}
		db.DPrintf(db.CACHEDSVCCLNT, "GetEndpoints nsrv: %v", csc.nsrv)
		// Update last endpoint version
		csc.lastEPV = v
	}
}

func (csc *CachedSvcClnt) getNServers() (int, error) {
	if csc.useEPCacheClnt {
		if csc.nsrv == 0 {
			instances, _, err := csc.epcc.GetEndpoints(csc.dir, epcache.NO_VERSION)
			if err != nil {
				return 0, err
			}
			return len(instances), nil
		}
		return csc.nsrv, nil
	}
	n, err := csc.dd.WaitEntriesN(1, true)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (csc *CachedSvcClnt) StatsSrvs() ([]*rpc.RPCStatsSnapshot, error) {
	n, err := csc.getNServers()
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
	if csc.useEPCacheClnt {
		instances, _, err := csc.epcc.GetEndpoints(csc.dir, epcache.NO_VERSION)
		if err != nil {
			return nil, err
		}
		istr := strconv.Itoa(i)
		var ep *sp.Tendpoint
		for _, is := range instances {
			if is.ID == istr {
				ep = sp.NewEndpointFromProto(is.EndpointProto)
			}
		}
		if ep == nil {
			db.DPrintf(db.ERROR, "No EP for cachesrv %v", i)
			return nil, fmt.Errorf("No EP for cachesrv %v", i)
		}
		return ep, nil
	}
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
	n, err := csc.getNServers()
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
	return cacheclnt.Key2shard(key, cache.NSHARD)
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

func (csc *CachedSvcClnt) GetBytes(key string) ([]byte, error) {
	return csc.GetBytesTraced(nil, key, false)
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
	n, err := csc.getNServers()
	if err != nil {
		return err
	}
	var srv string
	if backup {
		srv = csc.BackupServer(Key2server(key, n))
	} else {
		srv = csc.Server(Key2server(key, n))
	}
	db.DPrintf(db.CACHEDSVCCLNT, "Get key %v from srv %v", key, srv)
	return csc.cc.GetTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) PutBytesTraced(sctx *tproto.SpanContextConfig, key string, b []byte) error {
	n, err := csc.getNServers()
	if err != nil {
		return err
	}
	srv := csc.Server(Key2server(key, n))
	return csc.cc.PutBytesTracedFenced(sctx, srv, key, b, sp.NullFence())
}

func (csc *CachedSvcClnt) GetBytesTraced(sctx *tproto.SpanContextConfig, key string, backup bool) ([]byte, error) {
	n, err := csc.getNServers()
	if err != nil {
		return nil, err
	}
	var srv string
	if backup {
		srv = csc.BackupServer(Key2server(key, n))
	} else {
		srv = csc.Server(Key2server(key, n))
	}
	db.DPrintf(db.CACHEDSVCCLNT, "Get key %v from srv %v", key, srv)
	return csc.cc.GetBytesTracedFenced(sctx, srv, key, sp.NullFence())
}

func (csc *CachedSvcClnt) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	n, err := csc.getNServers()
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

func (csc *CachedSvcClnt) GetCacheClnt() *cacheclnt.CacheClnt {
	return csc.cc
}

func (csc *CachedSvcClnt) Close() {
	csc.done = true
	csc.dd.StopWatching()
}
