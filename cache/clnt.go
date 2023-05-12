package cache

import (
	"google.golang.org/protobuf/proto"

	tproto "sigmaos/tracing/proto"
)

type CacheClnt interface {
	Get(string, proto.Message) error
	Put(string, proto.Message) error
	Delete(string) error
	GetTraced(*tproto.SpanContextConfig, string, proto.Message) error
	PutTraced(*tproto.SpanContextConfig, string, proto.Message) error
	DeleteTraced(*tproto.SpanContextConfig, string) error
	IsMiss(error) bool
}
