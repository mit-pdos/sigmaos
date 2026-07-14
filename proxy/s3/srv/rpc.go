package srv

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proxy/s3/proto"
	rpcproto "sigmaos/rpc/proto"
)

type S3RpcAPI struct {
	mu        sync.Mutex
	fss3      *Fss3
	dataCache map[string]map[string][]byte
}

func newRPCAPI(fss3 *Fss3) *S3RpcAPI {
	return &S3RpcAPI{
		fss3:      fss3,
		dataCache: make(map[string]map[string][]byte),
	}
}

func (ra *S3RpcAPI) cachePut(bucket, key string, b []byte) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	bucketMap, ok := ra.dataCache[bucket]
	if !ok {
		bucketMap = make(map[string][]byte)
		ra.dataCache[bucket] = bucketMap
	}
	bucketMap[key] = b
}

func (ra *S3RpcAPI) cacheGet(bucket, key string) ([]byte, error) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	bucketMap, ok := ra.dataCache[bucket]
	if !ok {
		return nil, fmt.Errorf("bucket %v not found", bucket)
	}
	val, ok := bucketMap[key]
	if !ok {
		return nil, fmt.Errorf("key %v not found", key)
	}
	return val, nil
}

func (ra *S3RpcAPI) GetObject(ctx fs.CtxI, req proto.S3Req, rep *proto.S3Rep) error {
	db.DPrintf(db.S3, "GetObject RPC: bucket:%v key:%v cache:%v off:%v cnt:%v", req.Bucket, req.Key, req.Cache, req.Offset, req.Count)
	start := time.Now()
	clnt, err1 := ra.fss3.getClient(ctx)
	if err1 != nil {
		db.DPrintf(db.S3_ERR, "Err getClient: %v", err1)
		db.DPrintf(db.ERROR, "Err getClient: %v", err1)
		return err1
	}
	if req.Count > 0 {
		// Chunked read: [offset, offset+count). Bypasses the whole-object
		// cache. AWS truncates the range at EOF; a range starting at or
		// past EOF (InvalidRange) returns an empty blob, not an error.
		region := fmt.Sprintf("bytes=%d-%d", req.Offset, req.Offset+req.Count-1)
		input := &s3.GetObjectInput{
			Bucket: &req.Bucket,
			Key:    &req.Key,
			Range:  &region,
		}
		result, err := clnt.GetObject(context.TODO(), input)
		if err != nil {
			if strings.Contains(err.Error(), "InvalidRange") {
				rep.Blob = &rpcproto.Blob{Iov: [][]byte{{}}}
				rep.OK = true
				return nil
			}
			db.DPrintf(db.S3_ERR, "Err GetObject (ranged): %v", err)
			db.DPrintf(db.ERROR, "Err GetObject (ranged): %v", err)
			return err
		}
		nbyte := int(*result.ContentLength)
		rep.Blob = &rpcproto.Blob{
			Iov: [][]byte{make([]byte, nbyte)},
		}
		n, err := io.ReadAtLeast(result.Body, rep.Blob.Iov[0], nbyte)
		if n != nbyte || err != nil {
			db.DPrintf(db.S3_ERR, "Err Read (ranged): %v", err)
			db.DPrintf(db.ERROR, "Err Read (ranged): %v", err)
			return err
		}
		if err := result.Body.Close(); err != nil {
			db.DPrintf(db.S3_ERR, "Err Close: %v", err)
			db.DPrintf(db.ERROR, "Err Close: %v", err)
			return err
		}
		db.DPrintf(db.S3, "GetObject RPC success: bucket:%v key:%v off:%v nbyte (%v)", req.Bucket, req.Key, req.Offset, len(rep.Blob.Iov[0]))
		rep.OK = true
		return nil
	}
	b, err := ra.cacheGet(req.Bucket, req.Key)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Cache get miss: %v", err)
		db.DPrintf(db.ALWAYS, "S3 clnt get lat: %v", time.Since(start))
		input := &s3.GetObjectInput{
			Bucket: &req.Bucket,
			Key:    &req.Key,
		}
		result, err := clnt.GetObject(context.TODO(), input)
		if err != nil {
			db.DPrintf(db.S3_ERR, "Err GetObject: %v", err)
			db.DPrintf(db.ERROR, "Err GetObject: %v", err)
			return err
		}
		db.DPrintf(db.ALWAYS, "S3 Get getobject %v", time.Since(start))
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
		db.DPrintf(db.ALWAYS, "S3 Get read %v", time.Since(start))
		if err := result.Body.Close(); err != nil {
			db.DPrintf(db.S3_ERR, "Err Close: %v", err)
			db.DPrintf(db.ERROR, "Err Close: %v", err)
			return err
		}
		db.DPrintf(db.ALWAYS, "S3 Get close %v", time.Since(start))
		if req.Cache {
			db.DPrintf("Cache put %v %v len %v", req.Bucket, req.Key, len(rep.Blob.Iov[0]))
			ra.cachePut(req.Bucket, req.Key, rep.Blob.Iov[0])
		}
	} else {
		db.DPrintf(db.ALWAYS, "Cache get hit: %v", err)
		// Set up the reply IOVec
		rep.Blob = &rpcproto.Blob{
			Iov: [][]byte{b},
		}
	}
	db.DPrintf(db.S3, "GetObject RPC success: bucket:%v key:%v cache:%v nbyte (%v)", req.Bucket, req.Key, req.Cache, len(rep.Blob.Iov[0]))
	db.DPrintf(db.ALWAYS, "S3 Get e2e nbyte %v: %v", len(rep.Blob.Iov[0]), time.Since(start))
	return nil
}

// PutObject writes a whole object; clients that produce output
// incrementally (e.g. getput.GetPutWriter) buffer the full object and issue
// one PutObject on close. If chunked S3 puts are ever needed, this is where
// multipart-upload state would slot in: offset==0 starts a
// CreateMultipartUpload, each chunk is an UploadPart, and a final flag
// triggers CompleteMultipartUpload (S3 requires >=5MiB parts except the
// last).
func (ra *S3RpcAPI) PutObject(ctx fs.CtxI, req proto.S3Req, rep *proto.S3Rep) error {
	db.DPrintf(db.S3, "PutObject RPC: bucket:%v key:%v len:%v", req.Bucket, req.Key, len(req.Blob.Iov[0]))
	start := time.Now()
	clnt, err1 := ra.fss3.getClient(ctx)
	if err1 != nil {
		db.DPrintf(db.S3_ERR, "Err getClient: %v", err1)
		db.DPrintf(db.ERROR, "Err getClient: %v", err1)
		return err1
	}
	db.DPrintf(db.ALWAYS, "S3 clnt get(put) lat: %v", time.Since(start))
	input := &s3.PutObjectInput{
		Bucket: &req.Bucket,
		Key:    &req.Key,
		Body:   bytes.NewReader(req.Blob.Iov[0]),
	}
	if _, err := clnt.PutObject(context.TODO(), input); err != nil {
		db.DPrintf(db.S3_ERR, "Err PutObject: %v", err)
		db.DPrintf(db.ERROR, "Err PutObject: %v", err)
		return err
	}
	db.DPrintf(db.S3, "PutObject RPC success: bucket:%v key:%v len:%v", req.Bucket, req.Key, len(req.Blob.Iov[0]))
	db.DPrintf(db.ALWAYS, "S3 Put e2e: %v", time.Since(start))
	return nil
}
