package cache

import (
	"google.golang.org/protobuf/proto"
)

type CacheClnt interface {
	Get(string, proto.Message) error
	Put(string, proto.Message) error
	IsMiss(error) bool
}
