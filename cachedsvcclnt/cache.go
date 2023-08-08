package cachedsvcclnt

import (
	"google.golang.org/protobuf/proto"

	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

func (csc *CachedSvcClnt) Put(key string, val proto.Message) error {
	return csc.PutTraced(nil, key, val)
}

func (csc *CachedSvcClnt) Get(key string, val proto.Message) error {
	return csc.GetTraced(nil, key, val)
}

func (csc *CachedSvcClnt) Delete(key string) error {
	return csc.DeleteTraced(nil, key)
}

func (csc *CachedSvcClnt) GetTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	srv := csc.Server(key2server(key, csc.nServer()))
	return csc.cc.GetTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	srv := csc.Server(key2server(key, csc.nServer()))
	return csc.cc.PutTracedFenced(sctx, srv, key, val, sp.NullFence())
}

func (csc *CachedSvcClnt) DeleteTraced(sctx *tproto.SpanContextConfig, key string) error {
	srv := csc.Server(key2server(key, csc.nServer()))
	return csc.cc.DeleteTracedFenced(sctx, srv, key, sp.NullFence())
}
