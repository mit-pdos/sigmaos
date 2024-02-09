package sigmaclntclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	db "sigmaos/debug"
	"sigmaos/frame"
	rpcproto "sigmaos/rpc/proto"
	scproto "sigmaos/sigmaclntsrv/proto"
)

func TestBytes(t *testing.T) {
	iov := make(frame.IoVec, 1)
	iov[0] = make(frame.Tframe, 1024)
	msg := rpcproto.NewBlob(iov)
	req := scproto.SigmaWriteRequest{Blob: msg}
	blob := hasBlob(&req)
	assert.Nil(t, blob)
	db.DPrintf(db.TEST, "req %v", req)
	b, err := proto.Marshal(&req)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "b %d\n", len(b))
}
