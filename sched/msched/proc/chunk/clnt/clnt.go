package clnt

import (
	"time"

	db "sigmaos/debug"
	rpcproto "sigmaos/rpc/proto"
	shardedsvcrpcclnt "sigmaos/rpc/shardedsvc/clnt"
	proto "sigmaos/sched/msched/proc/chunk/proto"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

type ChunkClnt struct {
	*shardedsvcrpcclnt.ShardedSvcRPCClnt
	ch chan string
}

func NewChunkClnt(fsl *fslib.FsLib, eager bool) *ChunkClnt {
	db.DPrintf(db.CHUNKCLNT, "NewChunkClnt")
	var ch chan string
	if eager {
		ch = make(chan string)
	}
	ckclnt := &ChunkClnt{
		ShardedSvcRPCClnt: shardedsvcrpcclnt.NewShardedSvcRPCClntCh(fsl, sp.CHUNKD, ch, db.CHUNKCLNT, db.CHUNKCLNT_ERR),
		ch:                ch,
	}
	if eager {
		ckclnt.ShardedSvcRPCClnt.Init()
		go ckclnt.readCh()
	}
	return ckclnt
}

// Eagerly make chunk clnts
func (ckclnt *ChunkClnt) readCh() {
	for n := range ckclnt.ch {
		_, err := ckclnt.GetClnt(n)
		db.DPrintf(db.CHUNKCLNT, "new chunksrv: %v err %v\n", n, err)
	}
}

func (ckclnt *ChunkClnt) UnregisterSrv(srv string) {
	ckclnt.InvalidateEntry(srv)
}

func (ckclnt *ChunkClnt) GetFileStat(srvid, pn string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, paths []string, ep *sp.TendpointProto) (*sp.Tstat, string, error) {
	start := time.Now()
	rpcc, err := ckclnt.GetClnt(srvid)
	perf.LogSpawnLatency("ChunkClnt.GetFileStat.GetClnt %v", pid, perf.TIME_NOT_SET, start, paths)
	if err != nil {
		return nil, "", err
	}
	req := &proto.GetFileStatReq{
		Prog:               pn,
		RealmStr:           string(realm),
		Pid:                pid.String(),
		SigmaPath:          paths,
		S3Secret:           s3secret,
		NamedEndpointProto: ep,
	}
	res := &proto.GetFileStatRep{}
	if err := rpcc.RPC("ChunkSrv.GetFileStat", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT_ERR, "ChunkClnt.GetFileStat %v err %v", req, err)
		return nil, "", err
	}
	return sp.NewStatProto(res.Stat), res.Path, nil
}

// For chunksrv to fetch chunk from another chunksrv and return data in b
func (ckclnt *ChunkClnt) FetchChunk(srvid, pn string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, ck int, sz sp.Tsize, path []string, b []byte) (sp.Tsize, string, error) {
	rpcc, err := ckclnt.GetClnt(srvid)
	if err != nil {
		return 0, "", err
	}
	req := &proto.FetchChunkReq{
		Prog:      pn,
		ChunkId:   int32(ck),
		Size:      uint64(sz),
		Realm:     string(realm),
		SigmaPath: path,
		Pid:       pid.String(),
		Data:      true,
		S3Secret:  s3secret,
	}
	res := &proto.FetchChunkRep{}
	res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	if err := rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT_ERR, "ChunkClnt.FetchChunk %v err %v", req, err)
		return 0, "", err
	}
	return sp.Tsize(res.Size), res.Path, nil
}

// For uprocsrv to ask chunksrv to fetch ck, but not return data to uprocsrv
func (ckclnt *ChunkClnt) Fetch(srvid, prog string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, ck int, sz sp.Tsize, path []string, ep *sp.TendpointProto) (sp.Tsize, string, error) {
	rpcc, err := ckclnt.GetClnt(srvid)
	if err != nil {
		return 0, "", err
	}
	req := &proto.FetchChunkReq{
		Prog:               prog,
		ChunkId:            int32(ck),
		Size:               uint64(sz),
		Realm:              string(realm),
		SigmaPath:          path,
		Pid:                pid.String(),
		Data:               false,
		S3Secret:           s3secret,
		NamedEndpointProto: ep,
	}
	res := &proto.FetchChunkRep{}
	if err := rpcc.RPC("ChunkSrv.Fetch", req, res); err != nil {
		db.DPrintf(db.CHUNKCLNT_ERR, "ChunkClnt.Fetch %v err %v", req, err)
		return 0, "", err
	}
	return sp.Tsize(res.Size), res.Path, nil
}

func (ckclnt *ChunkClnt) FetchBinary(srvid, prog string, pid sp.Tpid, realm sp.Trealm, s3secret *sp.SecretProto, reqsz sp.Tsize, path []string, ep *sp.TendpointProto) (string, error) {
	n := (int64(reqsz) / sp.Conf.Chunk.CHUNK_SZ) + 1
	db.DPrintf(db.CHUNKCLNT, "FetchBinary %q %v %d", prog, reqsz, n)
	last := ""
	for ck := 0; ck < int(n); ck++ {
		if sz, path, err := ckclnt.Fetch(srvid, prog, pid, realm, s3secret, ck, reqsz, path, ep); err != nil {
			return "", err
		} else {
			db.DPrintf(db.CHUNKCLNT, "FetchBinary %q %d %v %q", prog, ck, sz, path)
			last = path
		}
	}
	return last, nil
}
