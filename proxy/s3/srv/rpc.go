package srv

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proxy/s3/proto"
	rpcproto "sigmaos/rpc/proto"
)

type rpcAPI struct {
	fss3 *Fss3
}

func newRPCAPI(fss3 *Fss3) *rpcAPI {
	return &rpcAPI{
		fss3: fss3,
	}
}

func (ra *rpcAPI) GetObject(ctx fs.CtxI, req proto.GetReq, rep *proto.GetRep) error {
	clnt, err1 := ra.fss3.getClient(ctx)
	if err1 != nil {
		db.DPrintf(db.S3_ERR, "Err getClient: %v", err1)
		db.DPrintf(db.ERROR, "Err getClient: %v", err1)
		return err1
	}
	input := &s3.GetObjectInput{
		Bucket: &req.Bucket,
		Key:    &req.Key,
	}
	result, err := clnt.GetObject(context.TODO(), input)
	if err != nil {
		db.DPrintf(db.S3_ERR, "Err getClient: %v", err)
		db.DPrintf(db.ERROR, "Err getClient: %v", err)
		return err
	}
	nbyte := int(*result.ContentLength)
	// Set up the reply IOVec
	rep.Blob = &rpcproto.Blob{
		Iov: [][]byte{make([]byte, nbyte)},
	}
	n, err := io.ReadAtLeast(result.Body, rep.Blob.Iov[0], nbyte)
	if n != nbyte || err != nil {
		db.DPrintf(db.S3_ERR, "Err Read: %v", err)
		db.DPrintf(db.ERROR, "Err Read: %v", err)
		return err
	}
	if err := result.Body.Close(); err != nil {
		db.DPrintf(db.S3_ERR, "Err Close: %v", err)
		db.DPrintf(db.ERROR, "Err Close: %v", err)
		return err
	}
	return nil
}
