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
	"sigmaos/sessdev"
	// sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

func MkKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

type CacheClnt struct {
	*cachedsvcclnt.CachedSvcClnt
	fsls   []*fslib.FsLib
	nshard uint32
}

func MkCacheClnt(fsls []*fslib.FsLib, job string, nshard uint32) (*CacheClnt, error) {
	cc := &CacheClnt{fsls: fsls, nshard: nshard}
	cc.fsls = fsls
	cg, err := cachedsvcclnt.MkCachedSvcClnt(fsls, CACHE)
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

func (cc *CacheClnt) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = cc.key2shard(key)

	b, err := proto.Marshal(val)
	if err != nil {
		return err
	}

	req.Value = b
	var res cacheproto.CacheResult
	if err := cc.RPC("CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Put(key string, val proto.Message) error {
	return cc.PutTraced(nil, key, val)
}

func (cc *CacheClnt) GetTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = cc.key2shard(key)
	s := time.Now()
	var res cacheproto.CacheResult
	if err := cc.RPC("CacheSrv.Get", req, &res); err != nil {
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

func (cc *CacheClnt) Get(key string, val proto.Message) error {
	return cc.GetTraced(nil, key, val)
}

func (cc *CacheClnt) DeleteTraced(sctx *tproto.SpanContextConfig, key string) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = cc.key2shard(key)
	var res cacheproto.CacheResult
	if err := cc.RPC("CacheSrv.Delete", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Delete(key string) error {
	return cc.DeleteTraced(nil, key)
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
