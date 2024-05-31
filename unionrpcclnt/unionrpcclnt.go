package unionrpcclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/dyndir"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	"sigmaos/sigmarpcchan"
)

type UnionRPCClnt struct {
	*dyndir.DynDir[*rpcclnt.RPCClnt]
}

func (urpcc *UnionRPCClnt) newEntry(n string) (*rpcclnt.RPCClnt, error) {
	ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{urpcc.FsLib}, filepath.Join(urpcc.Path, n))
	if err != nil {
		db.DPrintf(urpcc.ESelector, "Error NewSigmaRPCChan[srvID:%v]: %v", n, err)
		return nil, err
	}
	db.DPrintf(urpcc.LSelector, "newEntry NewSigmaRPCChan[srvID:%v]: %v", n, ch)
	rpcc := rpcclnt.NewRPCClnt(ch)
	return rpcc, nil
}

func NewUnionRPCClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *UnionRPCClnt {
	u := &UnionRPCClnt{}
	u.DynDir = dyndir.NewDynDir[*rpcclnt.RPCClnt](fsl, path, u.newEntry, lSelector, eSelector)
	return u
}

func (urpcc *UnionRPCClnt) Nsrv() (int, error) {
	return urpcc.Nentry()
}

func (urpcc *UnionRPCClnt) GetSrvs() ([]string, error) {
	return urpcc.GetEntries()
}

func (urpcc *UnionRPCClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	e, err := urpcc.GetEntry(srvID)
	if err != nil && serr.IsErrorNotfound(err) {
		// In some cases the caller knows that srvID exists, so force
		// an entry to be allocated.
		e1, err := urpcc.GetEntryAlloc(srvID)
		if err != nil {
			return nil, err
		}
		return e1, nil
	}
	return e, err
}

func (urpcc *UnionRPCClnt) UnregisterSrv(srvID string) bool {
	return urpcc.Remove(srvID)
}
