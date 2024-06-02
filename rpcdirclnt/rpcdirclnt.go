package rpcdirclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/dircache"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	"sigmaos/sigmarpcchan"
)

type RPCDirClnt struct {
	*dircache.DirCache[*rpcclnt.RPCClnt]
}

func (rpcdc *RPCDirClnt) newEntry(n string) (*rpcclnt.RPCClnt, error) {
	ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{rpcdc.FsLib}, filepath.Join(rpcdc.Path, n))
	if err != nil {
		db.DPrintf(rpcdc.ESelector, "Error NewSigmaRPCChan[srvID:%v]: %v", n, err)
		return nil, err
	}
	db.DPrintf(rpcdc.LSelector, "newEntry NewSigmaRPCChan[srvID:%v]: %v", n, ch)
	rpcc := rpcclnt.NewRPCClnt(ch)
	return rpcc, nil
}

func NewRPCDirClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *RPCDirClnt {
	u := &RPCDirClnt{}
	u.DirCache = dircache.NewDirCache[*rpcclnt.RPCClnt](fsl, path, u.newEntry, lSelector, eSelector)
	return u
}

func (rpcdc *RPCDirClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	e, err := rpcdc.GetEntry(srvID)
	if err != nil && serr.IsErrorNotfound(err) {
		// In some cases the caller knows that srvID exists, so force
		// an entry to be allocated.
		e1, err := rpcdc.GetEntryAlloc(srvID)
		if err != nil {
			return nil, err
		}
		return e1, nil
	}
	return e, err
}
