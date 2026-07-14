package clnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/proxy/s3/proto"
	rpcclnt "sigmaos/rpc/clnt"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/sigmaclnt/fslib"
)

type S3Clnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewS3ClntInit(fsl *fslib.FsLib, pn string, lazyInit bool) (*S3Clnt, error) {
	db.DPrintf(db.S3CLNT2, "New S3Clnt: %v", pn)
	rpcc, err := rpcclnt.NewRPCClnt(pn, sprpcclnt.WithSPChannel(fsl, lazyInit), sprpcclnt.WithDelegatedSPProxyChannel(fsl), rpcclntopts.WithShmem(fsl.GetShmemSegment()))
	if err != nil {
		return nil, err
	}
	return &S3Clnt{
		rpcc: rpcc,
	}, nil
}

func NewS3Clnt(fsl *fslib.FsLib, pn string) (*S3Clnt, error) {
	return NewS3ClntInit(fsl, pn, true)
}

func (clnt *S3Clnt) GetObject(bucket, key string, cache bool) ([]byte, error) {
	db.DPrintf(db.S3CLNT2, "GetObject bucket:%v key:%v", bucket, key)
	b := []byte{}
	var res proto.S3Rep
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{b},
	}
	req := &proto.S3Req{
		Bucket: bucket,
		Key:    key,
		Cache:  cache,
	}
	err := clnt.rpcc.RPC("S3RpcAPI.GetObject", req, &res)
	if err != nil {
		db.DPrintf(db.S3CLNT2_ERR, "Err GetObject: %v", err)
		db.DPrintf(db.ERROR, "Err GetObject: %v", err)
		return nil, err
	}
	db.DPrintf(db.S3CLNT2, "GetObject ok bucket:%v key:%v blob_len:%v", bucket, key, len(res.Blob.Iov))
	return res.Blob.Iov[0], nil
}

// GetObjectChunk reads cnt bytes starting at off. The returned buffer may be
// shorter than cnt if the range extends past the object's end (empty if off
// is at or past it). Ranged reads bypass the server's whole-object cache.
func (clnt *S3Clnt) GetObjectChunk(bucket, key string, off, cnt uint64) ([]byte, error) {
	db.DPrintf(db.S3CLNT2, "GetObjectChunk bucket:%v key:%v off:%v cnt:%v", bucket, key, off, cnt)
	b := []byte{}
	var res proto.S3Rep
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{b},
	}
	req := &proto.S3Req{
		Bucket: bucket,
		Key:    key,
		Offset: off,
		Count:  cnt,
	}
	err := clnt.rpcc.RPC("S3RpcAPI.GetObject", req, &res)
	if err != nil {
		db.DPrintf(db.S3CLNT2_ERR, "Err GetObjectChunk: %v", err)
		db.DPrintf(db.ERROR, "Err GetObjectChunk: %v", err)
		return nil, err
	}
	db.DPrintf(db.S3CLNT2, "GetObjectChunk ok bucket:%v key:%v off:%v len:%v", bucket, key, off, len(res.Blob.Iov[0]))
	return res.Blob.Iov[0], nil
}

func (clnt *S3Clnt) DelegatedGetObject(rpcIdx uint64) ([]byte, time.Duration, error) {
	db.DPrintf(db.S3CLNT2, "DelegatedGetObject(%v)", rpcIdx)
	b := []byte{}
	var res proto.S3Rep
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{b},
	}
	transferDur, err := clnt.rpcc.DelegatedRPC(rpcIdx, &res)
	if err != nil {
		db.DPrintf(db.S3CLNT2_ERR, "Err DelegatedGetObject: %v", err)
		db.DPrintf(db.ERROR, "Err DelegatedGetObject: %v", err)
		return nil, 0, err
	}
	db.DPrintf(db.S3CLNT2, "DelegatedGetObject(%v) ok blob_len:%v", rpcIdx, len(res.Blob.Iov))
	return res.Blob.Iov[0], transferDur, nil
}

func (clnt *S3Clnt) PutObject(bucket, key string, b []byte) error {
	db.DPrintf(db.S3CLNT2, "PutObject bucket:%v key:%v len:%v", bucket, key, len(b))
	var res proto.S3Rep
	req := &proto.S3Req{
		Bucket: bucket,
		Key:    key,
		Blob: &rpcproto.Blob{
			Iov: [][]byte{b},
		},
	}
	err := clnt.rpcc.RPC("S3RpcAPI.PutObject", req, &res)
	if err != nil {
		db.DPrintf(db.S3CLNT2_ERR, "Err PutObject: %v", err)
		db.DPrintf(db.ERROR, "Err PutObject: %v", err)
		return err
	}
	db.DPrintf(db.S3CLNT2, "PutObject ok bucket:%v key:%v len:%v", bucket, key, len(b))
	return nil
}

func (clnt *S3Clnt) DelegatedPutObject(rpcIdx uint64, bucket, key string, b []byte) error {
	db.DPrintf(db.S3CLNT2, "DelegatedPutObject bucket:%v key:%v len:%v", bucket, key, len(b))
	req := &proto.S3Req{
		Bucket: bucket,
		Key:    key,
		Blob: &rpcproto.Blob{
			Iov: [][]byte{b},
		},
	}
	err := clnt.rpcc.OutgoingDelegatedRPC(rpcIdx, "S3RpcAPI.PutObject", req)
	if err != nil {
		db.DPrintf(db.S3CLNT2_ERR, "Err DelegatedPutObject: %v", err)
		db.DPrintf(db.ERROR, "Err DelegatedPutObject: %v", err)
		return err
	}
	db.DPrintf(db.S3CLNT2, "DelegatedPutObject ok bucket:%v key:%v len:%v", bucket, key, len(b))
	return nil
}
