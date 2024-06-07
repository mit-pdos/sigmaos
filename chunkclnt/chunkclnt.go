package chunkclnt

import (
	"sigmaos/chunk"
	proto "sigmaos/chunk/proto"
	db "sigmaos/debug"
	"sigmaos/fslib"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcdirclnt"
	sp "sigmaos/sigmap"
)

type ChunkClnt struct {
	*rpcdirclnt.RPCDirClnt
	done int32
}

func NewChunkClnt(fsl *fslib.FsLib) *ChunkClnt {
	db.DPrintf(db.CHUNKCLNT, "NewChunkClnt")
	ckclnt := &ChunkClnt{
		RPCDirClnt: rpcdirclnt.NewRPCDirClnt(fsl, sp.CHUNKD, db.CHUNKCLNT, db.CHUNKCLNT_ERR),
	}
	return ckclnt
}

func (ckclnt *ChunkClnt) UnregisterSrv(srv string) {
	ckclnt.RPCDirClnt.RemoveEntry(srv)
}

func (ckclnt *ChunkClnt) GetFileStat(srvid, pn string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, paths []string) (*sp.Stat, string, error) {
	rpcc, err := ckclnt.RPCDirClnt.GetClnt(srvid)
	if err != nil {
		return nil, "", err
	}
	req := &proto.GetFileStatRequest{
		Prog:      pn,
		RealmStr:  string(realm),
		Pid:       pid.String(),
		SigmaPath: paths,
		S3Secret:  s3secret,
	}
	res := &proto.GetFileStatResponse{}
	if err := rpcc.RPC("ChunkSrv.GetFileStat", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT_ERR, "ChunkClnt.GetFileStat %v err %v", req, err)
		return nil, "", err
	}
	return sp.NewStatProto(res.Stat), res.Path, nil
}

// For chunksrv to fetch chunk from another chunksrv and return data in b
func (ckclnt *ChunkClnt) FetchChunk(srvid, pn string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, ck int, sz sp.Tsize, path []string, b []byte) (sp.Tsize, string, error) {
	rpcc, err := ckclnt.RPCDirClnt.GetClnt(srvid)
	if err != nil {
		return 0, "", err
	}
	req := &proto.FetchChunkRequest{
		Prog:      pn,
		ChunkId:   int32(ck),
		Size:      uint64(sz),
		Realm:     string(realm),
		SigmaPath: path,
		Pid:       pid.String(),
		Data:      true,
		S3Secret:  s3secret,
	}
	res := &proto.FetchChunkResponse{}
	res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	if err := rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT_ERR, "ChunkClnt.FetchChunk %v err %v", req, err)
		return 0, "", err
	}
	return sp.Tsize(res.Size), res.Path, nil
}

// For uprocsrv to ask chunksrv to fetch ck, but not return data to uprocsrv
func (ckclnt *ChunkClnt) Fetch(srvid, prog string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, ck int, sz sp.Tsize, path []string) (sp.Tsize, string, error) {
	rpcc, err := ckclnt.RPCDirClnt.GetClnt(srvid)
	if err != nil {
		return 0, "", err
	}
	req := &proto.FetchChunkRequest{
		Prog:      prog,
		ChunkId:   int32(ck),
		Size:      uint64(sz),
		Realm:     string(realm),
		SigmaPath: path,
		Pid:       pid.String(),
		Data:      false,
		S3Secret:  s3secret,
	}
	res := &proto.FetchChunkResponse{}
	if err := rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT_ERR, "ChunkClnt.Fetch %v err %v", req, err)
		return 0, "", err
	}
	return sp.Tsize(res.Size), res.Path, nil
}

func (ckclnt *ChunkClnt) FetchBinary(srvid, prog string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, reqsz sp.Tsize, path []string) (string, error) {
	n := (reqsz / chunk.CHUNKSZ) + 1
	db.DPrintf(db.CHUNKCLNT, "FetchBinary %q %v %d", prog, reqsz, n)
	last := ""
	for ck := 0; ck < int(n); ck++ {
		if sz, path, err := ckclnt.Fetch(srvid, prog, pid, realm, s3secret, ck, reqsz, path); err != nil {
			return "", err
		} else {
			db.DPrintf(db.CHUNKCLNT, "FetchBinary %q %d %v %q", prog, ck, sz, path)
			last = path
		}
	}
	return last, nil
}
