package cachedsvcclnt

import (
	"hash/fnv"
	"strconv"
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/cache"
	"sigmaos/cacheclnt"
	"sigmaos/cachedsvc"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpc"
	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

func key2server(key string, nserver int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	server := int(h.Sum32()) % nserver
	return server
}

type CachedSvcClnt struct {
	sync.Mutex
	fsl  *fslib.FsLib
	cc   *cacheclnt.CacheClnt
	pn   string
	srvs map[string]struct{}
	rdr  *fslib.FdReader
}

func NewCachedSvcClnt(fsls []*fslib.FsLib, job string) (*CachedSvcClnt, error) {
	csc := &CachedSvcClnt{
		fsl:  fsls[0],
		pn:   cache.CACHE,
		cc:   cacheclnt.NewCacheClnt(fsls, job, cachesrv.NSHARD),
		srvs: make(map[string]struct{}),
	}
	sts, err := csc.fsl.GetDir(csc.srvDir())
	if err != nil {
		return nil, err
	}
	csc.addServer(sts)
	if err := csc.setWatch(); err != nil {
		return nil, err
	}
	return csc, nil
}

func (csc *CachedSvcClnt) srvDir() string {
	return csc.pn + cachedsvc.SVRDIR
}

func (csc *CachedSvcClnt) setWatch() error {
	dir := csc.srvDir()
	_, rdr, err := csc.fsl.ReadDir(dir)
	if err != nil {
		return err
	}
	csc.rdr = rdr
	if err := csc.fsl.SetDirWatch(csc.rdr.Fd(), dir, csc.Watch); err != nil {
		return err
	}
	return nil
}

func (csc *CachedSvcClnt) addServer(sts []*sp.Stat) {
	csc.Lock()
	defer csc.Unlock()

	for _, st := range sts {
		if _, ok := csc.srvs[st.Name]; !ok {
			csc.srvs[st.Name] = struct{}{}
		}
	}
}

func (csc *CachedSvcClnt) Watch(path string, err error) {
	db.DPrintf(db.CACHEDSVCCLNT, "cachedsvcclnt watch %v err %v", path, err)
	if err != nil {
		db.DPrintf(db.CACHEDSVCCLNT, "Watch err %v\n", err)
		return
	}
	sts, err := csc.fsl.GetDir(path)
	if len(sts) > len(csc.srvs) {
		csc.addServer(sts)
	}
	csc.rdr.Close()
	if err := csc.setWatch(); err != nil {
		db.DPrintf(db.CACHEDSVCCLNT, "setWatch err %v\n", err)
	}
}

func (csc *CachedSvcClnt) Server(i int) string {
	return csc.pn + cachedsvc.Server(strconv.Itoa(i))
}

func (csc *CachedSvcClnt) nServer() int {
	csc.Lock()
	defer csc.Unlock()
	return len(csc.srvs)
}

func (csc *CachedSvcClnt) StatsSrvs() ([]*rpc.SigmaRPCStats, error) {
	n := csc.nServer()
	stats := make([]*rpc.SigmaRPCStats, 0, n)
	for i := 0; i < n; i++ {
		st, err := csc.cc.StatsSrv(csc.Server(i))
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, nil
}

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

func (csc *CachedSvcClnt) StatsClnt() []map[string]*rpc.MethodStat {
	return csc.StatsClnt()
}

func (csc *CachedSvcClnt) Dump(g int) (map[string]string, error) {
	srv := csc.Server(g)
	return csc.cc.DumpSrv(srv)
}
