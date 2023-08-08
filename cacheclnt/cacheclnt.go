package cacheclnt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"io"
	"path"
	"reflect"
	"strconv"

	"time"

	"google.golang.org/protobuf/proto"

	"sigmaos/cache"
	cacheproto "sigmaos/cache/proto"
	"sigmaos/cachedsvcclnt"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	"sigmaos/sessdev"
	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

func MkKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

type CacheClnt struct {
	csc    *cachedsvcclnt.CachedSvcClnt
	rpcc   *rpcclnt.ClntCache
	fsl    *fslib.FsLib
	nshard uint32
}

func MkCacheClnt(fsls []*fslib.FsLib, job string, nshard uint32) (*CacheClnt, error) {
	cc := &CacheClnt{fsl: fsls[0], nshard: nshard}
	cg, err := cachedsvcclnt.MkCachedSvcClnt(fsls, CACHE)
	if err != nil {
		return nil, err
	}
	cc.csc = cg
	return cc, nil
}

func NewCacheClntRPC(fsls []*fslib.FsLib, job string, nshard uint32) (*CacheClnt, error) {
	cc := &CacheClnt{fsl: fsls[0], nshard: nshard, rpcc: rpcclnt.NewRPCClntCache(fsls)}
	return cc, nil
}

func (cc *CacheClnt) key2shard(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := h.Sum32() % cc.nshard
	return shard
}

func (cc *CacheClnt) PutTraced(sctx *tproto.SpanContextConfig, srv, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
		Fence:             sp.NullFence().FenceProto(),
		Key:               key,
		Shard:             cc.key2shard(key),
	}

	b, err := proto.Marshal(val)
	if err != nil {
		return err
	}

	req.Value = b
	var res cacheproto.CacheResult
	if err := cc.csc.RPC("CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Put(key string, val proto.Message) error {
	return cc.PutTraced(nil, "", key, val)
}

func (cc *CacheClnt) PutSrv(srv, key string, val proto.Message) error {
	return cc.PutTraced(nil, srv, key, val)
}

func (cc *CacheClnt) AppendFence(srv, key string, val proto.Message, f *sp.Tfence) error {
	b, err := proto.Marshal(val)
	if err != nil {
		return err
	}
	l := uint32(len(b))
	var buf bytes.Buffer
	wr := bufio.NewWriter(&buf)
	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.LittleEndian, b); err != nil {
		return err
	}
	wr.Flush()
	req := &cacheproto.CacheRequest{
		Key:   key,
		Shard: cc.key2shard(key),
		Mode:  uint32(sp.OAPPEND),
		Value: buf.Bytes(),
		Fence: f.FenceProto(),
	}
	var res cacheproto.CacheResult
	if err := cc.rpcc.RPC(srv, "CacheSrv.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) GetTraced(sctx *tproto.SpanContextConfig, srv, key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = cc.key2shard(key)
	s := time.Now()
	var res cacheproto.CacheResult
	if err := cc.csc.RPC("CacheSrv.Get", req, &res); err != nil {
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
	return cc.GetTraced(nil, "", key, val)
}

func (cc *CacheClnt) GetSrv(srv, key string, val proto.Message) error {
	return cc.GetTraced(nil, "", key, val)
}

func (c *CacheClnt) GetVals(srv, key string, m proto.Message, f *sp.Tfence) ([]proto.Message, error) {
	req := &cacheproto.CacheRequest{
		Key:   key,
		Shard: c.key2shard(key),
		Fence: f.FenceProto(),
	}
	s := time.Now()
	var res cacheproto.CacheResult
	if err := c.rpcc.RPC(srv, "CacheSrv.Get", req, &res); err != nil {
		return nil, err
	}
	if time.Since(s) > 150*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache getvals: %v", time.Since(s))
	}
	typ := reflect.TypeOf(m)
	vals := make([]proto.Message, 0)
	rdr := bytes.NewReader(res.Value)
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

func (cc *CacheClnt) DeleteTraced(sctx *tproto.SpanContextConfig, srv, key string) error {
	req := &cacheproto.CacheRequest{
		SpanContextConfig: sctx,
	}
	req.Key = key
	req.Shard = cc.key2shard(key)
	var res cacheproto.CacheResult
	if err := cc.csc.RPC("CacheSrv.Delete", req, &res); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Delete(key string) error {
	return cc.DeleteTraced(nil, "", key)
}

func (cc *CacheClnt) DeleteSrv(srv, key string) error {
	return cc.DeleteTraced(nil, "", key)
}

func (c *CacheClnt) CreateShard(srv string, shard cache.Tshard, fence *sp.Tfence, vals map[string][]byte) error {
	req := &cacheproto.ShardArg{
		Shard: uint32(shard),
		Fence: fence.FenceProto(),
		Vals:  vals,
	}
	var res cacheproto.CacheOK
	if err := c.rpcc.RPC(srv, "CacheSrv.CreateShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) DeleteShard(srv string, shard cache.Tshard, f *sp.Tfence) error {
	req := &cacheproto.ShardArg{
		Shard: uint32(shard),
		Fence: f.FenceProto(),
	}
	var res cacheproto.CacheOK
	if err := c.rpcc.RPC(srv, "CacheSrv.DeleteShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) FreezeShard(srv string, shard cache.Tshard, f *sp.Tfence) error {
	req := &cacheproto.ShardArg{
		Shard: uint32(shard),
		Fence: f.FenceProto(),
	}
	var res cacheproto.CacheOK
	if err := c.rpcc.RPC(srv, "CacheSrv.FreezeShard", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) DumpShard(srv string, shard cache.Tshard, f *sp.Tfence) (map[string][]byte, error) {
	req := &cacheproto.ShardArg{
		Shard: uint32(shard),
		Fence: f.FenceProto(),
	}
	var res cacheproto.CacheDump
	if err := c.rpcc.RPC(srv, "CacheSrv.DumpShard", req, &res); err != nil {
		return nil, err
	}
	return res.Vals, nil
}

func (cc *CacheClnt) StatsSrv() ([]*rpc.SigmaRPCStats, error) {
	return cc.csc.StatsSrv()
}

func (cc *CacheClnt) Dump(g int) (map[string]string, error) {
	srv := cc.csc.Server(g)
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

func (cc *CacheClnt) DumpSrv(srv string) (map[string]string, error) {
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
