package cache

import (
	"errors"
	"fmt"

	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"sigmaos/serr"
	tproto "sigmaos/tracing/proto"
)

type Tkey string

func (k Tkey) String() string {
	return string(k)
}

func MkKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

type Tshard int

func (s Tshard) String() string {
	return fmt.Sprintf("s-%03d", s)
}

type CacheClnt interface {
	Get(string, proto.Message) error
	Put(string, proto.Message) error
	Delete(string) error
	GetTraced(*tproto.SpanContextConfig, string, proto.Message) error
	PutTraced(*tproto.SpanContextConfig, string, proto.Message) error
	DeleteTraced(*tproto.SpanContextConfig, string) error
}

func IsMiss(err error) bool {
	var sr *serr.Err
	if errors.As(err, &sr) {
		return sr.IsErrNotfound() && strings.HasPrefix(sr.Obj, "key")
	}
	return false
}

func IsMissShard(err error) bool {
	var sr *serr.Err
	if errors.As(err, &sr) {
		return sr.IsErrNotfound() && strings.HasPrefix(sr.Obj, "shard")
	}
	return false
}
