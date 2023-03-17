package cacheclnt

import (
	"hash/fnv"
	"strconv"

	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/cache/proto"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/protdev"
	"sigmaos/reader"
	"sigmaos/sessdev"
	"sigmaos/shardsvcclnt"
	sp "sigmaos/sigmap"
)

var (
	ErrMiss = cachesrv.ErrMiss
)

func MkKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

func key2shard(key string, nshard int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := int(h.Sum32()) % nshard
	return shard
}

type CacheClnt struct {
	*shardsvcclnt.ShardSvcClnt
	fsl *fslib.FsLib
}

func MkCacheClnt(fsl *fslib.FsLib, job string) (*CacheClnt, error) {
	cc := &CacheClnt{}
	cc.fsl = fsl
	cg, err := shardsvcclnt.MkShardSvcClnt(fsl, sp.CACHE, cc.Watch)
	if err != nil {
		return nil, err
	}
	cc.ShardSvcClnt = cg
	return cc, nil
}

func (cc *CacheClnt) IsMiss(err error) bool {
	return err.Error() == ErrMiss.Error()
}

func (cc *CacheClnt) Watch(path string, nshard int, err error) {
	db.DPrintf(db.ALWAYS, "CacheClnt watch %v %d err %v\n", path, nshard, err)
}

func (cc *CacheClnt) RPC(m string, arg *cacheproto.CacheRequest, res *cacheproto.CacheResult) error {
	n := key2shard(arg.Key, cc.Nshard())
	return cc.ShardSvcClnt.RPC(n, m, arg, res)
}

func (c *CacheClnt) Put(key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{}
	req.Key = key

	b, err := proto.Marshal(val)
	if err != nil {
		return err
	}

	req.Value = b
	var res cacheproto.CacheResult
	if err := c.RPC("Cache.Put", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Get(key string, val proto.Message) error {
	req := &cacheproto.CacheRequest{}
	req.Key = key
	var res cacheproto.CacheResult
	if err := c.RPC("Cache.Get", req, &res); err != nil {
		return err
	}
	if err := proto.Unmarshal(res.Value, val); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Dump(g int) (map[string]string, error) {
	srv := cc.Server(g)
	b, err := cc.fsl.GetFile(srv + "/" + sessdev.CloneName(cachesrv.DUMP))
	if err != nil {
		return nil, err
	}
	sid := string(b)
	sidn := sessdev.SidName(sid, cachesrv.DUMP)
	fn := srv + "/" + sidn + "/" + sessdev.DataName(cachesrv.DUMP)
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

func (cc *CacheClnt) StatsSrv() ([]*protdev.SigmaRPCStats, error) {
	n := cc.Nshard()
	stats := make([]*protdev.SigmaRPCStats, 0, n)
	for i := 0; i < n; i++ {
		st, err := cc.ShardSvcClnt.StatsSrv(i)
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, nil
}

func (cc *CacheClnt) StatsClnt() []*protdev.RPCStats {
	n := cc.Nshard()
	stats := make([]*protdev.RPCStats, 0, n)
	for i := 0; i < n; i++ {
		stats = append(stats, cc.ShardSvcClnt.StatsClnt(i))
	}
	return stats
}

//
// stubs to make cache-clerk compile
//

func (cc *CacheClnt) GetReader(key string) (*reader.Reader, error) {
	return nil, nil
}

func (c *CacheClnt) Append(key string, val any) error {
	return nil
}
