package cacheclnt

import (
	"hash/fnv"
	"path"
	"strconv"

	"time"

	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/cache/proto"
	"sigmaos/cachedsvcclnt"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpc"
	"sigmaos/sessdev"
	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

func MkKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

func key2server(key string, nserver int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	server := int(h.Sum32()) % nserver
	return server
}

type CacheClnt struct {
	*cachedsvcclnt.CachedSvcClnt
	fsls   []*fslib.FsLib
	nshard uint32
}

func MkCacheClnt(fsls []*fslib.FsLib, job string, nshard uint32) (*CacheClnt, error) {
	cc := &CacheClnt{fsls: fsls, nshard: nshard}
	cc.fsls = fsls
	cg, err := cachedsvcclnt.MkCachedSvcClnt(fsls, CACHE, cc.Watch)
	if err != nil {
		return nil, err
	}
	cc.CachedSvcClnt = cg
	return cc, nil
}

func (cc *CacheClnt) key2shard(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := h.Sum32() % cc.nshard
	return shard
}

func (cc *CacheClnt) Watch(path string, nshard int, err error) {
	db.DPrintf(db.ALWAYS, "CacheClnt watch %v %d err %v\n", path, nshard, err)
}

func (cc *CacheClnt) RPC(m string, arg *cacheproto.CacheRequest, res *cacheproto.CacheResult) error {
	n := key2server(arg.Key, cc.NServer())
	arg.Fence = sp.NullFence().FenceProto()
	return cc.CachedSvcClnt.RPC(n, m, arg, res)
}

func (c *CacheClnt) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = c.key2shard(key)

	b, err := proto.Marshal(val)
	if err != nil {
		return err
	}

	req.Value = b
	var res cacheproto.CacheResult
	if err := c.RPC("CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Put(key string, val proto.Message) error {
	return c.PutTraced(nil, key, val)
}

func (c *CacheClnt) GetTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = c.key2shard(key)
	s := time.Now()
	var res cacheproto.CacheResult
	if err := c.RPC("CacheSrv.Get", req, &res); err != nil {
		return err
	}
	if time.Since(s) > 150*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache get: %v", time.Since(s))
	}
	if err := proto.Unmarshal(res.Value, val); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Get(key string, val proto.Message) error {
	return c.GetTraced(nil, key, val)
}

func (c *CacheClnt) DeleteTraced(sctx *tproto.SpanContextConfig, key string) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = c.key2shard(key)
	var res cacheproto.CacheResult
	if err := c.RPC("CacheSrv.Delete", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Delete(key string) error {
	return c.DeleteTraced(nil, key)
}

func (cc *CacheClnt) Dump(g int) (map[string]string, error) {
	srv := cc.Server(g)
	dir := path.Join(srv, cachesrv.DUMP)
	b, err := cc.fsls[0].GetFile(dir + "/" + sessdev.CLONE)
	if err != nil {
		return nil, err
	}
	sid := string(b)
	fn := dir + "/" + sid + "/" + sessdev.DATA
	b, err = cc.fsls[0].GetFile(fn)
	if err != nil {
		return nil, err
	}
	dump := &cacheproto.CacheDump{}
	if err := proto.Unmarshal(b, dump); err != nil {
		return nil, err
	}
	m := map[string]string{}
	for k, v := range dump.Vals {
		m[k] = string(v)
	}
	return m, nil
}

func (cc *CacheClnt) StatsSrv() ([]*rpc.SigmaRPCStats, error) {
	n := cc.NServer()
	stats := make([]*rpc.SigmaRPCStats, 0, n)
	for i := 0; i < n; i++ {
		st, err := cc.CachedSvcClnt.StatsSrv(i)
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, nil
}

func (cc *CacheClnt) StatsClnt() []map[string]*rpc.MethodStat {
	n := cc.NServer()
	stats := make([]map[string]*rpc.MethodStat, 0, n)
	for i := 0; i < n; i++ {
		stats = append(stats, cc.CachedSvcClnt.StatsClnt(i))
	}
	return stats
}
