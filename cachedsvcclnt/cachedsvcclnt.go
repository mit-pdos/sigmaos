package cachedsvcclnt

import (
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/cachedsvc"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/reader"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
)

type ServerWatch func(string, int, error)

type CachedSvcClnt struct {
	sync.Mutex
	fsls  []*fslib.FsLib
	clnts []*rpcclnt.RPCClnt
	pn    string
	sw    ServerWatch
	rdr   *reader.Reader
}

func MkCachedSvcClnt(fsls []*fslib.FsLib, pn string, sw ServerWatch) (*CachedSvcClnt, error) {
	csc := &CachedSvcClnt{
		fsls: fsls,
		pn:   pn,
		sw:   sw,
	}
	sts, err := csc.fsls[0].GetDir(csc.srvDir())
	if err != nil {
		return nil, err
	}
	n := len(sts)
	csc.clnts = make([]*rpcclnt.RPCClnt, 0)
	for i := 0; i < n; i++ {
		if err := csc.addClnt(i); err != nil {
			return nil, err
		}
	}
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
	_, rdr, err := csc.fsls[0].ReadDir(dir)
	if err != nil {
		return err
	}
	csc.rdr = rdr
	if err := csc.fsls[0].SetDirWatch(csc.rdr.Fid(), dir, csc.Watch); err != nil {
		return err
	}
	return nil
}

func (csc *CachedSvcClnt) addClnt(i int) error {
	csc.Lock()
	defer csc.Unlock()

	sn := csc.pn + cachedsvc.Server(i)
	rpcc, err := rpcclnt.MkRPCClnt(csc.fsls, sn)
	if err != nil {
		return err
	}
	csc.clnts = append(csc.clnts, rpcc)
	return nil
}

func (csc *CachedSvcClnt) Watch(path string, err error) {
	db.DPrintf(db.CACHEDSVCCLNT, "%v: cachedsvcclnt watch %v err %v\n", proc.GetName(), path, err)
	if err != nil {
		db.DPrintf(db.CACHEDSVCCLNT, "Watch err %v\n", err)
		return
	}
	sts, err := csc.fsls[0].GetDir(path)
	if len(sts) > len(csc.clnts) {
		if err := csc.addClnt(len(sts) - 1); err != nil {
			db.DPrintf(db.CACHEDSVCCLNT, "%v: addClnt err %v\n", proc.GetName(), err)
		}
		csc.sw(path, len(sts), err)
	}
	csc.rdr.Close()
	if err := csc.setWatch(); err != nil {
		db.DPrintf(db.CACHEDSVCCLNT, "setWatch err %v\n", err)
	}
}

func (csc *CachedSvcClnt) Server(i int) string {
	return csc.pn + cachedsvc.Server(i)
}

func (csc *CachedSvcClnt) NServer() int {
	csc.Lock()
	defer csc.Unlock()
	return len(csc.clnts)
}

func (csc *CachedSvcClnt) RPC(i int, m string, arg proto.Message, res proto.Message) error {
	return csc.clnts[i].RPC(m, arg, res)
}

func (csc *CachedSvcClnt) StatsSrv(i int) (*rpc.SigmaRPCStats, error) {
	return csc.clnts[i].StatsSrv()
}

func (csc *CachedSvcClnt) StatsClnt(i int) map[string]*rpc.MethodStat {
	return csc.clnts[i].StatsClnt()
}
