package clnt

import (
	"path/filepath"

	db "sigmaos/debug"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dircache"
)

type AllocFn func(string)

type ShardedSvcRPCClnt struct {
	*dircache.DirCache[*rpcclnt.RPCClnt]
	allocFn AllocFn // Callback to be invoked when a new client is created
}

func (rpcdc *ShardedSvcRPCClnt) newClnt(n string) (*rpcclnt.RPCClnt, error) {
	pn := filepath.Join(rpcdc.Path, n)
	rpcc, err := sprpcclnt.NewRPCClnt(rpcdc.FsLib, pn)
	if err != nil {
		db.DPrintf(rpcdc.ESelector, "Error NewSigmaRPCClnt[srvID:%v]: %v", pn, err)
		return nil, err
	}
	db.DPrintf(rpcdc.LSelector, "newClnt NewSigmaRPCClnt[srvID:%v]: %v", pn, rpcc)
	if rpcdc.allocFn != nil {
		rpcdc.allocFn(n)
	}
	return rpcc, nil
}

func NewShardedSvcRPCClntAllocFn(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector, fn AllocFn) *ShardedSvcRPCClnt {
	u := &ShardedSvcRPCClnt{
		allocFn: fn,
	}
	u.DirCache = dircache.NewDirCache[*rpcclnt.RPCClnt](fsl, path, u.newClnt, nil, lSelector, eSelector)
	return u
}

func NewShardedSvcRPCClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *ShardedSvcRPCClnt {
	return NewShardedSvcRPCClntAllocFn(fsl, path, lSelector, eSelector, nil)
}

func NewShardedSvcRPCClntCh(fsl *fslib.FsLib, path string, ch chan string, lSelector db.Tselector, eSelector db.Tselector) *ShardedSvcRPCClnt {
	u := &ShardedSvcRPCClnt{}
	u.DirCache = dircache.NewDirCache[*rpcclnt.RPCClnt](fsl, path, u.newClnt, ch, lSelector, eSelector)
	return u
}

func NewShardedSvcRPCClntFilter(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector, filters []string) *ShardedSvcRPCClnt {
	u := &ShardedSvcRPCClnt{}
	u.DirCache = dircache.NewDirCacheFilter[*rpcclnt.RPCClnt](fsl, path, u.newClnt, nil, lSelector, eSelector, filters)
	return u
}

func (rpcdc *ShardedSvcRPCClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	return rpcdc.GetEntry(srvID)
}
