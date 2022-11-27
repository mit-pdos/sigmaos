package cacheclnt

import (
	"encoding/json"
	"hash/fnv"
	"strconv"

	"sigmaos/clonedev"
	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/hotel"
	np "sigmaos/ninep"
	"sigmaos/protdevclntgrp"
	"sigmaos/sessdev"
)

func key2shard(key string, nshard int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := int(h.Sum32()) % nshard
	return shard
}

type CacheClnt struct {
	*protdevclntgrp.ClntGroup
	fsl *fslib.FsLib
}

func MkCacheClnt(fsl *fslib.FsLib, n int) (*CacheClnt, error) {
	cc := &CacheClnt{}
	cg, err := protdevclntgrp.MkProtDevClntGrp(fsl, n)
	if err != nil {
		return nil, err
	}
	cc.fsl = fsl
	cc.ClntGroup = cg
	return cc, nil
}

func (cc *CacheClnt) RPC(m string, arg hotel.CacheRequest, res any) error {
	n := key2shard(arg.Key, cc.Nshard())
	return cc.ClntGroup.RPC(n, m, arg, res)
}

func (cc *CacheClnt) Dump(g int) (map[string]string, error) {
	gn := group.GRP + strconv.Itoa(g)

	b, err := cc.fsl.GetFile(np.HOTELCACHE + gn + "/" + clonedev.CloneName(hotel.DUMP))
	if err != nil {
		return nil, err
	}
	sid := string(b)

	sidn := clonedev.SidName(sid, hotel.DUMP)
	fn := np.HOTELCACHE + gn + "/" + sidn + "/" + sessdev.DataName(hotel.DUMP)
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
