package cache

import (
	"google.golang.org/protobuf/proto"

	tproto "sigmaos/tracing/proto"
)

type CacheClnt interface {
	Get(string, proto.Message) error
	Put(string, proto.Message) error
	GetTraced(*tproto.SpanContextConfig, string, proto.Message) error
	PutTraced(*tproto.SpanContextConfig, string, proto.Message) error
	IsMiss(error) bool
}
