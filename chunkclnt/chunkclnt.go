package chunkclnt

import (
	proto "sigmaos/chunk/proto"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
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

func (ckclnt *ChunkClnt) FetchChunk(pn, pid string, realm sp.Trealm, ck int, sz sp.Tsize, b []byte) (sp.Tsize, error) {
	req := &proto.FetchChunkRequest{
		Prog:    pn,
		ChunkId: int32(ck),
		Size:    uint64(sz),
		Realm:   string(realm),
		Pid:     pid,
	}
	res := &proto.FetchChunkResponse{}
	res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	if err := ckclnt.rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT, "ChunkClnt.FetchChunk %v err %v", req, err)
		return 0, err
	}
	return sp.Tsize(res.Size), nil
}

func (ckclnt *ChunkClnt) Fetch(p *proc.Proc, realm sp.Trealm, ck int, sz sp.Tsize) (sp.Tsize, error) {
	req := &proto.FetchChunkRequest{
		Prog:      p.GetProgram(),
		ChunkId:   int32(ck),
		Size:      uint64(sz),
		Realm:     string(realm),
		SigmaPath: p.GetSigmaPath(),
		Pid:       p.GetPid().String(),
	}
	res := &proto.FetchChunkResponse{}
	if err := ckclnt.rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT, "ChunkClnt.Fetch %v err %v", req, err)
		return 0, err
	}
	return sp.Tsize(res.Size), nil
}
