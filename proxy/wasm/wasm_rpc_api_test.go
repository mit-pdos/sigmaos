package wasm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/apps/cache/proto"
	db "sigmaos/debug"
)

type TestRPCAPI struct {
	*testing.T
	cacheMultiGetReqs map[int]*cacheproto.CacheMultiGetReq
}

func NewTestRPCAPI(t *testing.T, cacheMultiGetReqs map[int]*cacheproto.CacheMultiGetReq) *TestRPCAPI {
	return &TestRPCAPI{
		T:                 t,
		cacheMultiGetReqs: cacheMultiGetReqs,
	}
}

func (ts *TestRPCAPI) Send(rpcIdx uint64, pn string, b []byte) error {
	db.DPrintf(db.TEST, "Send pn:%v", pn)
	cacheMultiGet := &cacheproto.CacheMultiGetReq{}
	if err := proto.Unmarshal(b, cacheMultiGet); !assert.Nil(ts.T, err, "Err unmarshal MultiGet: %v", err) {
		return err
	}
	// Get the "ground truth" request
	groundTruthReq := ts.cacheMultiGetReqs[int(rpcIdx)]
	// Check that the number of gets matches
	if !assert.Equal(ts.T, len(groundTruthReq.Gets), len(cacheMultiGet.Gets), "Mismatched number of get requests for rpcIdx: %v", rpcIdx) {
		return fmt.Errorf("Mismatched number of get requests for rpcIdx: %v", rpcIdx)
	}
	for i := range groundTruthReq.Gets {
		// Check that the key matches
		if !assert.Equal(ts.T, groundTruthReq.Gets[i].Key, cacheMultiGet.Gets[i].Key, "Mismatched key for rpcIdx: %v getIdx: %v", rpcIdx, i) {
			return fmt.Errorf("Mismatched key for rpcIdx: %v getIdx: %v", rpcIdx, i)
		}
		// Check that the shard matches
		// Check that the key matches
		if !assert.Equal(ts.T, groundTruthReq.Gets[i].Shard, cacheMultiGet.Gets[i].Shard, "Mismatched shard for rpcIdx: %v getIdx: %v", rpcIdx, i) {
			return fmt.Errorf("Mismatched shard for rpcIdx: %v getIdx: %v", rpcIdx, i)
		}
	}
	return nil
}

func (ts *TestRPCAPI) Recv(rpcIdx uint64) ([]byte, error) {
	db.DFatalf("Unimplemented")
	return nil, nil
}
