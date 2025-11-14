package clnt

import (
	db "sigmaos/debug"
	"sigmaos/proxy/s3/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/sigmaclnt/fslib"
)

type S3Clnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewS3Clnt(fsl *fslib.FsLib, pn string) (*S3Clnt, error) {
	db.DPrintf(db.S3CLNT2, "New S3Clnt: %v", pn)
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, pn)
	if err != nil {
		return nil, err
	}
	return &S3Clnt{
		rpcc: rpcc,
	}, nil
}

func (clnt *S3Clnt) GetObject(bucket, key string) ([]byte, error) {
	db.DPrintf(db.S3CLNT2, "GetObject bucket:%v key:%v", bucket, key)
	b := []byte{}
	var res proto.GetRep
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{b},
	}
	req := &proto.GetReq{
		Bucket: bucket,
		Key:    key,
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
