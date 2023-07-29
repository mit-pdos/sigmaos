package kv

import (
	"hash/fnv"
	"path"
	"time"

	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/cache/proto"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/sessdev"
	tproto "sigmaos/tracing/proto"
)

var (
	ErrMiss = cachesrv.ErrMiss
)

type CacheClnt struct {
	fsl    *fslib.FsLib
	rpcc   *rpcclnt.ClntCache
	nshard uint32
}

func NewCacheClnt(fsl *fslib.FsLib, nshard uint32) *CacheClnt {
	return &CacheClnt{
		fsl:    fsl,
		rpcc:   rpcclnt.NewRPCClntCache(fsl),
		nshard: nshard,
	}
}

func (cc *CacheClnt) IsMiss(err error) bool {
	return err.Error() == ErrMiss.Error()
}

func (cc *CacheClnt) key2shard(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := h.Sum32() % cc.nshard
	return shard
}

func (cc *CacheClnt) RPC(srv, m string, arg *cacheproto.CacheRequest, res *cacheproto.CacheResult) error {
	return cc.rpcc.RPC(srv, m, arg, res)
}

func (c *CacheClnt) PutTraced(sctx *tproto.SpanContextConfig, srv, key string, val proto.Message) error {
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
	if err := c.RPC(srv, "CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Put(srv, key string, val proto.Message) error {
	return c.PutTraced(nil, srv, key, val)
}

func (c *CacheClnt) GetTraced(sctx *tproto.SpanContextConfig, srv, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = c.key2shard(key)
	s := time.Now()
	var res cacheproto.CacheResult
	if err := c.RPC(srv, "CacheSrv.Get", req, &res); err != nil {
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

func (c *CacheClnt) Get(srv, key string, val proto.Message) error {
	return c.GetTraced(nil, srv, key, val)
}

func (c *CacheClnt) DeleteTraced(sctx *tproto.SpanContextConfig, srv, key string) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = c.key2shard(key)
	var res cacheproto.CacheResult
	if err := c.RPC(srv, "CacheSrv.Delete", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Delete(srv, key string) error {
	return c.DeleteTraced(nil, srv, key)
}

func (c *CacheClnt) CreateShard(srv string, shard uint32) error {
	req := &cacheproto.ShardArg{
		Shard: shard,
	}
	var res cacheproto.CacheOK
	if err := c.rpcc.RPC(srv, "CacheSrv.CreateShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) DeleteShard(srv string, shard uint32) error {
	req := &cacheproto.ShardArg{
		Shard: shard,
	}
	var res cacheproto.CacheOK
	if err := c.rpcc.RPC(srv, "CacheSrv.DeleteShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) DumpShard(srv string, shard uint32) (map[string][]byte, error) {
	req := &cacheproto.ShardArg{
		Shard: shard,
	}
	var res cacheproto.CacheDump
	if err := c.rpcc.RPC(srv, "CacheSrv.DumpShard", req, &res); err != nil {
		return nil, err
	}
	return res.Vals, nil
}

func (c *CacheClnt) FillShard(srv string, shard uint32, m map[string][]byte) error {
	req := &cacheproto.ShardFill{
		Shard: shard,
		Vals:  m,
	}
	var res cacheproto.CacheOK
	if err := c.rpcc.RPC(srv, "CacheSrv.FillShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Dump(srv string) (map[string]string, error) {
	dir := path.Join(srv, cachesrv.DUMP)
	b, err := cc.fsl.GetFile(dir + "/" + sessdev.CLONE)
	if err != nil {
		return nil, err
	}
	sid := string(b)
	fn := dir + "/" + sid + "/" + sessdev.DATA
	b, err = cc.fsl.GetFile(fn)
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
