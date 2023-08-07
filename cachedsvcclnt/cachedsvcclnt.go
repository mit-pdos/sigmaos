package cachedsvcclnt

import (
	"hash/fnv"
	"strconv"
	"sync"

	cacheproto "sigmaos/cache/proto"

	"sigmaos/cachedsvc"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/reader"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
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
	rpcc *rpcclnt.ClntCache
	pn   string
	srvs map[string]struct{}
	rdr  *reader.Reader
}

func MkCachedSvcClnt(fsls []*fslib.FsLib, pn string) (*CachedSvcClnt, error) {
	csc := &CachedSvcClnt{
		fsl:  fsls[0],
		pn:   pn,
		rpcc: rpcclnt.NewRPCClntCache(fsls),
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
	if err := csc.fsl.SetDirWatch(csc.rdr.Fid(), dir, csc.Watch); err != nil {
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
	db.DPrintf(db.CACHEDSVCCLNT, "%v: cachedsvcclnt watch %v err %v\n", proc.GetName(), path, err)
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

func (csc *CachedSvcClnt) RPC(m string, arg *cacheproto.CacheRequest, res *cacheproto.CacheResult) error {
	pn := csc.Server(key2server(arg.Key, csc.nServer()))
	arg.Fence = sp.NullFence().FenceProto()
	return csc.rpcc.RPC(pn, m, arg, res)
}

func (csc *CachedSvcClnt) nServer() int {
	csc.Lock()
	defer csc.Unlock()
	return len(csc.srvs)
}

func (csc *CachedSvcClnt) StatsSrv() ([]*rpc.SigmaRPCStats, error) {
	n := csc.nServer()
	stats := make([]*rpc.SigmaRPCStats, 0, n)
	for i := 0; i < n; i++ {
		st, err := csc.rpcc.StatsSrv(csc.Server(i))
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, nil
}

func (csc *CachedSvcClnt) StatsClnt() []map[string]*rpc.MethodStat {
	return csc.rpcc.StatsClnt()
}
