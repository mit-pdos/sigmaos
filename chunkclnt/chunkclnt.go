package chunkclnt

import (
	proto "sigmaos/chunk/proto"
	db "sigmaos/debug"
	"sigmaos/fslib"
	rpcproto "sigmaos/rpc/proto"
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

func (ckclnt *ChunkClnt) FetchChunk(pn, pid string, realm sp.Trealm, ck int, sz sp.Tsize, path []string, b []byte) (sp.Tsize, error) {
	req := &proto.FetchChunkRequest{
		Prog:      pn,
		ChunkId:   int32(ck),
		Size:      uint64(sz),
		Realm:     string(realm),
		SigmaPath: path,
		Pid:       pid,
		Data:      true,
	}
	res := &proto.FetchChunkResponse{}
	res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	if err := ckclnt.rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT, "ChunkClnt.FetchChunk %v err %v", req, err)
		return 0, err
	}
	return sp.Tsize(res.Size), nil
}

func (ckclnt *ChunkClnt) Fetch(prog string, pid sp.Tpid, realm sp.Trealm, ck int, sz sp.Tsize, path []string) (sp.Tsize, error) {
	req := &proto.FetchChunkRequest{
		Prog:      prog,
		ChunkId:   int32(ck),
		Size:      uint64(sz),
		Realm:     string(realm),
		SigmaPath: path,
		Pid:       pid.String(),
		Data:      false,
	}
	res := &proto.FetchChunkResponse{}
	if err := ckclnt.rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT, "ChunkClnt.Fetch %v err %v", req, err)
		return 0, err
	}
	return sp.Tsize(res.Size), nil
}
