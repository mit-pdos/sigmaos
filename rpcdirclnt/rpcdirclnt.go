package rpcdirclnt

import (
	"path/filepath"
	"time"

	db "sigmaos/debug"
	"sigmaos/dircache"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/sigmarpcchan"
)

type AllocFn func(string)

type RPCDirClnt struct {
	*dircache.DirCache[*rpcclnt.RPCClnt]
	allocFn AllocFn // Callback to be invoked when a new client is created
}

func (rpcdc *RPCDirClnt) newClnt(n string) (*rpcclnt.RPCClnt, error) {
	s := time.Now()
	pn := filepath.Join(rpcdc.Path, n)
	rpcc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{rpcdc.FsLib}, pn)
	if err != nil {
		db.DPrintf(rpcdc.ESelector, "Error NewSigmaRPCClnt[srvID:%v]: %v", pn, err)
		return nil, err
	}
	db.DPrintf(rpcdc.LSelector, "newClnt NewSigmaRPCClnt[srvID:%v]: %v", pn, rpcc)
	db.DPrintf(db.SPAWN_LAT, "newClnt %v lat %v", pn, time.Since(s))
	if rpcdc.allocFn != nil {
		rpcdc.allocFn(n)
	}
	return rpcc, nil
}

func NewRPCDirClntAllocFn(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector, fn AllocFn) *RPCDirClnt {
	u := &RPCDirClnt{
		allocFn: fn,
	}
	u.DirCache = dircache.NewDirCache[*rpcclnt.RPCClnt](fsl, path, u.newClnt, lSelector, eSelector)
	return u
}

func NewRPCDirClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *RPCDirClnt {
	return NewRPCDirClntAllocFn(fsl, path, lSelector, eSelector, nil)
}

func NewRPCDirClntFilter(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector, filter string) *RPCDirClnt {
	u := &RPCDirClnt{}
	u.DirCache = dircache.NewDirCacheFilter[*rpcclnt.RPCClnt](fsl, path, u.newClnt, lSelector, eSelector, filter)
	return u
}

func (rpcdc *RPCDirClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	return rpcdc.GetEntry(srvID)
}
