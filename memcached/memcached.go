package memcached

import (
	"hash/fnv"
	"net"

	"github.com/bradfitz/gomemcache/memcache"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

type MemcachedClnt struct {
	cc *memcache.Client
}

func NewMemcachedClnt(fsl *fslib.FsLib, job string) (*MemcachedClnt, error) {
	var addrs []string
	err := fsl.GetFileJson(sp.MEMCACHED, &addrs)
	if err != nil {
		db.DFatalf("Error get memcache addr file: %v", err)
	}
	mc := &MemcachedClnt{
		memcache.NewFromSelector(newServerSelector(addrs)),
	}
	mc.cc.MaxIdleConns = 8000
	return mc, nil
}

func (mc *MemcachedClnt) Get(key string, m proto.Message) error {
	i, err := mc.cc.Get(key)
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(i.Value, m); err != nil {
		return err
	}
	return nil
}

func (mc *MemcachedClnt) Put(key string, m proto.Message) error {
	b, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	return mc.cc.Set(&memcache.Item{
		Key:   key,
		Value: b,
	})
}

func (mc *MemcachedClnt) Delete(key string) error {
	return mc.cc.Delete(key)
}

func (mc *MemcachedClnt) GetTraced(s *tproto.SpanContextConfig, key string, m proto.Message) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (mc *MemcachedClnt) PutTraced(s *tproto.SpanContextConfig, key string, m proto.Message) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (mc *MemcachedClnt) DeleteTraced(s *tproto.SpanContextConfig, key string) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (mc *MemcachedClnt) IsMiss(e error) bool {
	return e.Error() == memcache.ErrCacheMiss.Error()
}

func key2server(key string, nserver int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	server := int(h.Sum32()) % nserver
	return server
}

type serverSelector struct {
	addrs []net.Addr
}

func newServerSelector(addrs []string) *serverSelector {
	// TCP or UDP?
	as := make([]net.Addr, 0, len(addrs))
	for _, addr := range addrs {
		a, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			db.DFatalf("Error resovlve TCP addr")
		}
		as = append(as, a)
	}
	return &serverSelector{as}
}

func (ss *serverSelector) PickServer(key string) (net.Addr, error) {
	return ss.addrs[key2server(key, len(ss.addrs))], nil
}

func (ss *serverSelector) Each(f func(net.Addr) error) error {
	for _, a := range ss.addrs {
		f(a)
	}
	return nil
}
