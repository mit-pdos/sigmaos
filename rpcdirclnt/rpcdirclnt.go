package rpcdirclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/dircache"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/sigmarpcchan"
)

type RPCDirClnt struct {
	*dircache.DirCache[*rpcclnt.RPCClnt]
}

func (rpcdc *RPCDirClnt) newEntry(n string) (*rpcclnt.RPCClnt, error) {
	rpcc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{rpcdc.FsLib}, filepath.Join(rpcdc.Path, n))
	if err != nil {
		db.DPrintf(rpcdc.ESelector, "Error NewSigmaRPCClnt[srvID:%v]: %v", n, err)
		return nil, err
	}
	db.DPrintf(rpcdc.LSelector, "newEntry NewSigmaRPCClnt[srvID:%v]: %v", n, rpcc)
	return rpcc, nil
}

func NewRPCDirClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *RPCDirClnt {
	u := &RPCDirClnt{}
	u.DirCache = dircache.NewDirCache[*rpcclnt.RPCClnt](fsl, path, u.newEntry, lSelector, eSelector)
	return u
}

func NewRPCDirClntFilter(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector, filter string) *RPCDirClnt {
	u := &RPCDirClnt{}
	u.DirCache = dircache.NewDirCacheFilter[*rpcclnt.RPCClnt](fsl, path, u.newEntry, lSelector, eSelector, filter)
	return u
}

func (rpcdc *RPCDirClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	return rpcdc.GetEntry(srvID)
}
