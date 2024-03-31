package chunkclnt

import (
	"time"

	proto "sigmaos/chunksrv/proto"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type ChunkClnt struct {
	*fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewChunkClnt(fsl *fslib.FsLib, pn string) (*ChunkClnt, error) {
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{fsl}, pn)
	if err != nil {
		return nil, err
	}
	ckclnt := &ChunkClnt{
		FsLib: fsl,
		rpcc:  rpcc,
	}
	return ckclnt, nil
}

func (ckclnt *ChunkClnt) UprocdFetch(pn string, realm sp.Trealm, ck int, sz sp.Tsize, path []string, b []byte) (sp.Tsize, error) {
	s := time.Now()
	req := &proto.FetchRequest{
		Prog:    pn,
		ChunkId: int32(ck),
		Size:    uint64(sz),
		Realm:   string(realm),
		Path:    path,
	}
	res := &proto.FetchResponse{}
	if err := ckclnt.rpcc.RPC("UprocSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT, "UprocSrv.Fetch %v err %v", req, err)
		return 0, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Fetch latency %v", pn, time.Since(s))
	return sp.Tsize(res.Size), nil
}
