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
	ents, err := urpcc.GetEntries()
	if err != nil {
		return 0, err
	}
	return len(ents), nil
}

func (urpcc *UnionRPCClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	e, ok := urpcc.GetEntry(srvID)
	if !ok {
		// In some cases the caller knows that srvID exists, so force
		// an entry to be allocated.
		e1, err := urpcc.GetEntryAlloc(srvID)
		if err != nil {
			return nil, err
		}
		return e1, nil
	}
	return e, nil
}

func (urpcc *UnionRPCClnt) UnregisterSrv(srvID string) bool {
	return urpcc.Remove(srvID)
}

// Get the next server, round-robin.
func (urpcc *UnionRPCClnt) NextSrv() (string, error) {
	n, ok := urpcc.RoundRobin()
	if !ok {
		return "", serr.NewErr(serr.TErrNotfound, "no next server")
	}
	return n, nil
}

// Get the next server, randomly.
func (urpcc *UnionRPCClnt) RandomSrv() (string, error) {
	n, ok := urpcc.Random()
	if !ok {
		return "", serr.NewErr(serr.TErrNotfound, "no random server")
	}
	return n, nil
}

func (urpcc *UnionRPCClnt) GetSrvs() ([]string, error) {
	return urpcc.GetEntries()
}
