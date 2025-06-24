package clnt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"io"
	"path/filepath"
	"reflect"
	"strconv"

	"time"

	"google.golang.org/protobuf/proto"

	"sigmaos/apps/cache"
	cacheproto "sigmaos/apps/cache/proto"
	db "sigmaos/debug"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	rpcdev "sigmaos/rpc/dev"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	tproto "sigmaos/util/tracing/proto"
)

func NewKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

type CacheClnt struct {
	*rpcclnt.ClntCache
	fsl    *fslib.FsLib
	nshard int
}

func NewCacheClnt(fsl *fslib.FsLib, job string, nshard int) *CacheClnt {
	cc := &CacheClnt{
		fsl:       fsl,
		nshard:    nshard,
		ClntCache: rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
	}
	return cc
}

func (cc *CacheClnt) Key2shard(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := h.Sum32() % uint32(cc.nshard)
	return shard
}

func (cc *CacheClnt) NewPutBytes(sctx *tproto.SpanContextConfig, key string, b []byte, f *sp.Tfence) (*cacheproto.CacheReq, error) {
	return &cacheproto.CacheReq{
		SpanContextConfig: sctx,
		Fence:             f.FenceProto(),
		Key:               key,
		Shard:             cc.Key2shard(key),
		Value:             b,
	}, nil
}

func (cc *CacheClnt) NewPut(sctx *tproto.SpanContextConfig, key string, val proto.Message, f *sp.Tfence) (*cacheproto.CacheReq, error) {
	b, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	return cc.NewPutBytes(sctx, key, b, f)
}

func (cc *CacheClnt) PutTracedFenced(sctx *tproto.SpanContextConfig, srv, key string, val proto.Message, f *sp.Tfence) error {
	req, err := cc.NewPut(sctx, key, val, f)
	if err != nil {
		return err
	}
	var res cacheproto.CacheRep
	if err := cc.RPC(srv, "CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) PutBytesTracedFenced(sctx *tproto.SpanContextConfig, srv, key string, b []byte, f *sp.Tfence) error {
	req, err := cc.NewPutBytes(sctx, key, b, f)
	if err != nil {
		return err
	}
	var res cacheproto.CacheRep
	if err := cc.RPC(srv, "CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) PutSrv(srv, key string, val proto.Message) error {
	return cc.PutTracedFenced(nil, srv, key, val, sp.NullFence())
}

func (cc *CacheClnt) PutSrvFenced(srv, key string, val proto.Message, f *sp.Tfence) error {
	return cc.PutTracedFenced(nil, srv, key, val, f)
}

func (cc *CacheClnt) NewAppend(key string, val proto.Message, f *sp.Tfence) (*cacheproto.CacheReq, error) {
	b, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	l := uint32(len(b))
	var buf bytes.Buffer
	wr := bufio.NewWriter(&buf)
	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return nil, err
	}
	if err := binary.Write(wr, binary.LittleEndian, b); err != nil {
		return nil, err
	}
	wr.Flush()
	return &cacheproto.CacheReq{
		Key:   key,
		Shard: cc.Key2shard(key),
		Mode:  uint32(sp.OAPPEND),
		Value: buf.Bytes(),
		Fence: f.FenceProto(),
	}, nil
}

func (cc *CacheClnt) AppendFence(srv, key string, val proto.Message, f *sp.Tfence) error {
	req, err := cc.NewAppend(key, val, f)
	if err != nil {
		return err
	}
	var res cacheproto.CacheRep
	if err := cc.RPC(srv, "CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) NewGet(sctx *tproto.SpanContextConfig, key string, f *sp.Tfence) *cacheproto.CacheReq {
	return &cacheproto.CacheReq{
		SpanContextConfig: sctx,
		Key:               key,
		Shard:             cc.Key2shard(key),
		Fence:             f.FenceProto(),
	}
}

func (cc *CacheClnt) GetTracedFenced(sctx *tproto.SpanContextConfig, srv, key string, val proto.Message, f *sp.Tfence) error {
	req := cc.NewGet(sctx, key, f)
	s := time.Now()
	var res cacheproto.CacheRep
	if err := cc.RPC(srv, "CacheSrv.Get", req, &res); err != nil {
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

func (cc *CacheClnt) GetSrv(srv, key string, val proto.Message, f *sp.Tfence) error {
	return cc.GetTracedFenced(nil, srv, key, val, f)
}

func ReadVals(m proto.Message, b []byte) ([]proto.Message, error) {
	typ := reflect.TypeOf(m)
	vals := make([]proto.Message, 0)
	rdr := bytes.NewReader(b)
	for {
		var l uint32
		if err := binary.Read(rdr, binary.LittleEndian, &l); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		b := make([]byte, int(l))
		if _, err := io.ReadFull(rdr, b); err != nil {
			return nil, err
		}
		val := reflect.New(typ.Elem()).Interface().(proto.Message)
		if err := proto.Unmarshal(b, val); err != nil {
		}
		vals = append(vals, val)
	}
	return vals, nil
}

func (cc *CacheClnt) GetVals(srv, key string, m proto.Message, f *sp.Tfence) ([]proto.Message, error) {
	req := cc.NewGet(nil, key, f)
	s := time.Now()
	var res cacheproto.CacheRep
	if err := cc.RPC(srv, "CacheSrv.Get", req, &res); err != nil {
		return nil, err
	}
	if time.Since(s) > 150*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache getvals: %v", time.Since(s))
	}
	return ReadVals(m, res.Value)
}

func (cc *CacheClnt) DeleteTracedFenced(sctx *tproto.SpanContextConfig, srv, key string, f *sp.Tfence) error {
	req := &cacheproto.CacheReq{
		SpanContextConfig: sctx,
		Fence:             f.FenceProto(),
		Key:               key,
		Shard:             cc.Key2shard(key),
	}
	var res cacheproto.CacheRep
	if err := cc.RPC(srv, "CacheSrv.Delete", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) DeleteSrv(srv, key string) error {
	return cc.DeleteTracedFenced(nil, srv, key, sp.NullFence())
}

func (c *CacheClnt) NewShardReq(shard cache.Tshard, fence *sp.Tfence, vals cache.Tcache) *cacheproto.ShardReq {
	return &cacheproto.ShardReq{
		Shard: uint32(shard),
		Fence: fence.FenceProto(),
		Vals:  vals,
	}
}

func (c *CacheClnt) CreateShard(srv string, shard cache.Tshard, fence *sp.Tfence, vals cache.Tcache) error {
	req := c.NewShardReq(shard, fence, vals)
	var res cacheproto.CacheOK
	if err := c.RPC(srv, "CacheSrv.CreateShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) DeleteShard(srv string, shard cache.Tshard, f *sp.Tfence) error {
	req := &cacheproto.ShardReq{
		Shard: uint32(shard),
		Fence: f.FenceProto(),
	}
	var res cacheproto.CacheOK
	if err := c.RPC(srv, "CacheSrv.DeleteShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) FreezeShard(srv string, shard cache.Tshard, f *sp.Tfence) error {
	req := &cacheproto.ShardReq{
		Shard: uint32(shard),
		Fence: f.FenceProto(),
	}
	var res cacheproto.CacheOK
	if err := c.RPC(srv, "CacheSrv.FreezeShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) DumpShard(srv string, shard cache.Tshard, f *sp.Tfence) (cache.Tcache, error) {
	req := &cacheproto.ShardReq{
		Shard: uint32(shard),
		Fence: f.FenceProto(),
	}
	var res cacheproto.ShardData
	if err := c.RPC(srv, "CacheSrv.DumpShard", req, &res); err != nil {
		return nil, err
	}
	return res.Vals, nil
}

func (cc *CacheClnt) StatsSrv(srv string) (*rpc.RPCStatsSnapshot, error) {
	return cc.ClntCache.StatsSrv(srv)
}

func (cc *CacheClnt) StatsClnt() []map[string]*rpc.MethodStatSnapshot {
	return cc.ClntCache.StatsClnt()
}

func (cc *CacheClnt) DumpSrv(srv string) (map[string]string, error) {
	dir := filepath.Join(srv, cache.DUMP)
	b, err := cc.fsl.GetFile(dir + "/" + rpcdev.CLONE)
	if err != nil {
		return nil, err
	}
	sid := string(b)
	fn := dir + "/" + sid + "/" + rpcdev.DATA
	b, err = cc.fsl.GetFile(fn)
	if err != nil {
		return nil, err
	}
	data := &cacheproto.ShardData{}
	if err := proto.Unmarshal(b, data); err != nil {
		return nil, err
	}
	m := map[string]string{}
	for k, v := range data.Vals {
		m[k] = string(v)
	}
	return m, nil
}
