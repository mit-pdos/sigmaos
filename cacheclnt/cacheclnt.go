package cacheclnt

import (
	"encoding/json"
	"hash/fnv"
	"strconv"

	"sigmaos/cachesrv"
	"sigmaos/cachesrv/proto"
	"sigmaos/clonedev"
	"sigmaos/fslib"
	"sigmaos/group"
	np "sigmaos/ninep"
	"sigmaos/protdevclntgrp"
	"sigmaos/protdevsrv"
	"sigmaos/sessdev"
)

var (
	ErrMiss = cachesrv.ErrMiss
)

func key2shard(key string, nshard int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := int(h.Sum32()) % nshard
	return shard
}

type CacheClnt struct {
	*protdevclntgrp.ClntGroup
	fsl    *fslib.FsLib
	nshard int
}

func MkCacheClnt(fsl *fslib.FsLib, n int) (*CacheClnt, error) {
	cc := &CacheClnt{}
	cc.nshard = n
	cg, err := protdevclntgrp.MkProtDevClntGrp(fsl, np.HOTELCACHE, n)
	if err != nil {
		return nil, err
	}
	cc.fsl = fsl
	cc.ClntGroup = cg
	return cc, nil
}

func (cc *CacheClnt) RPC(m string, arg *proto.CacheRequest, res *proto.CacheResult) error {
	n := key2shard(arg.Key, cc.Nshard())
	return cc.ClntGroup.RPC(n, m, arg, res)
}

func (c *CacheClnt) Set(key string, val any) error {
	req := &proto.CacheRequest{}
	req.Key = key
	b, err := json.Marshal(val)
	if err != nil {
		return nil
	}
	req.Value = b
	var res proto.CacheResult
	if err := c.RPC("Cache.Set", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Get(key string, val any) error {
	req := &proto.CacheRequest{}
	req.Key = key
	var res proto.CacheResult
	if err := c.RPC("Cache.Get", req, &res); err != nil {
		return err
	}
	if err := json.Unmarshal(res.Value, val); err != nil {
		return err
	}
	return nil
}

func (cc *CacheClnt) Dump(g int) (map[string]string, error) {
	gn := group.GRP + strconv.Itoa(g)

	b, err := cc.fsl.GetFile(np.HOTELCACHE + gn + "/" + clonedev.CloneName(cachesrv.DUMP))
	if err != nil {
		return nil, err
	}
	sid := string(b)
	sidn := clonedev.SidName(sid, cachesrv.DUMP)
	fn := np.HOTELCACHE + gn + "/" + sidn + "/" + sessdev.DataName(cachesrv.DUMP)
	b, err = cc.fsl.GetFile(fn)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (cc *CacheClnt) StatsSrv() ([]*protdevsrv.Stats, error) {
	stats := make([]*protdevsrv.Stats, 0, cc.nshard)
	for i := 0; i < cc.nshard; i++ {
		st, err := cc.ClntGroup.StatsSrv(i)
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, nil
}

func (cc *CacheClnt) StatsClnt() []*protdevsrv.Stats {
	stats := make([]*protdevsrv.Stats, 0, cc.nshard)
	for i := 0; i < cc.nshard; i++ {
		stats = append(stats, cc.ClntGroup.StatsClnt(i))
	}
	return stats
}
