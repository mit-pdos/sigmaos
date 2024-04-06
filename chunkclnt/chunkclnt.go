package chunkclnt

import (
	proto "sigmaos/chunksrv/proto"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
)

type ChunkClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewChunkClnt(fsl *fslib.FsLib, pn string) (*ChunkClnt, error) {
	db.DPrintf(db.CHUNKCLNT, "NewChunkClnt %q", pn)
	ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{fsl}, pn)
	if err != nil {
		db.DPrintf(db.ERROR, "rpcclnt err %v", err)
		return nil, err
	}
	ckclnt := &ChunkClnt{
		rpcc: rpcclnt.NewRPCClnt(ch),
	}
	return ckclnt, nil
}

func (ckclnt *ChunkClnt) FetchChunk(pn string, realm sp.Trealm, ck int, sz sp.Tsize, b []byte) (sp.Tsize, error) {
	req := &proto.FetchChunkRequest{
		Prog:    pn,
		ChunkId: int32(ck),
		Size:    uint64(sz),
		Realm:   string(realm),
	}
	res := &proto.FetchChunkResponse{}
	if err := ckclnt.rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT, "ChunkSrv.Fetch %v err %v", req, err)
		return 0, err
	}
	return sp.Tsize(res.Size), nil
}
