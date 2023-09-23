package unionrpcclnt

import (
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type UnionRPCClnt struct {
	*fslib.FsLib
	sync.Mutex
	path      string
	clnts     map[string]*rpcclnt.RPCClnt
	srvs      []string
	rrOffset  int
	lSelector db.Tselector
	eSelector db.Tselector
}

func NewUnionRPCClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *UnionRPCClnt {
	u := &UnionRPCClnt{
		FsLib:     fsl,
		path:      path,
		clnts:     make(map[string]*rpcclnt.RPCClnt),
		srvs:      make([]string, 0),
		lSelector: lSelector,
		eSelector: eSelector,
	}
	go u.monitorSrvs()
	return u
}

func (urpcc *UnionRPCClnt) Nsrv() (int, error) {
	sds, err := urpcc.GetSrvs()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
}

func (urpcc *UnionRPCClnt) GetClnt(srvID string) (*rpcclnt.RPCClnt, error) {
	urpcc.Lock()
	defer urpcc.Unlock()

	var rpcc *rpcclnt.RPCClnt
	var ok bool
	if rpcc, ok = urpcc.clnts[srvID]; !ok {
		var err error
		rpcc, err = rpcclnt.NewRPCClnt([]*fslib.FsLib{urpcc.FsLib}, path.Join(urpcc.path, srvID))
		if err != nil {
			db.DPrintf(urpcc.eSelector, "Error newRPCClnt[srvID:%v]: %v", srvID, err)
			return nil, err
		}
		urpcc.clnts[srvID] = rpcc
	}
	return rpcc, nil
}

// Update the list of active procds.
func (urpcc *UnionRPCClnt) UpdateSrvs(force bool) {
	urpcc.Lock()
	defer urpcc.Unlock()

	// If the caller is not forcing an update, and the list of servers has
	// already been populated, do nothing and return.
	if !force && len(urpcc.srvs) > 0 {
		db.DPrintf(urpcc.lSelector, "No need to update srv list")
		return
	}
	// Read the procd union dir.
	srvs, err := urpcc.GetSrvs()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error ReadDir procd: %v", err)
		return
	}
	urpcc.updateSrvsL(srvs)
}

func (urpcc *UnionRPCClnt) updateSrvsL(srvs []string) {
	db.DPrintf(urpcc.lSelector, "Update srvs %v", srvs)
	// Alloc enough space for the list of srvs.
	urpcc.srvs = make([]string, 0, len(srvs))
	for _, srvid := range srvs {
		urpcc.srvs = append(urpcc.srvs, srvid)
	}
}

func (urpcc *UnionRPCClnt) UnregisterSrv(srvID string) {
	urpcc.Lock()
	defer urpcc.Unlock()

	delete(urpcc.clnts, srvID)
	for i := 0; i < len(urpcc.srvs); i++ {
		if urpcc.srvs[i] == srvID {
			urpcc.srvs = append(urpcc.srvs[:i], urpcc.srvs[i+1:]...)
			break
		}
	}
}

// Get the next server, round-robin.
func (urpcc *UnionRPCClnt) NextSrv() (string, error) {
	urpcc.Lock()
	defer urpcc.Unlock()

	if len(urpcc.srvs) == 0 {
		return "", serr.NewErr(serr.TErrNotfound, "no srvs to spawn on")
	}

	srvID := urpcc.srvs[urpcc.rrOffset%len(urpcc.srvs)]
	urpcc.rrOffset++
	return srvID, nil
}

func (urpcc *UnionRPCClnt) GetSrvs() ([]string, error) {
	sts, err := urpcc.GetDir(urpcc.path)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

// Monitor for changes to the set of servers listed in the union directory.
func (urpcc *UnionRPCClnt) monitorSrvs() {
	for {
		sts, err := urpcc.ReadDirWatch(urpcc.path, func(sts []*sp.Stat) bool {
			// Construct a map of the service IDs in the union dir.
			srvsMap := map[string]bool{}
			for _, srvID := range sp.Names(sts) {
				srvsMap[srvID] = true
			}

			urpcc.Lock()
			defer urpcc.Unlock()

			// If the lengths don't match, the union dir has changed. Return false to
			// stop reading the dir and return into monitorSrvs.
			if len(sts) != len(urpcc.srvs) {
				return false
			}
			for _, srvID := range urpcc.srvs {
				// If a service is not present in the updated list of service IDs, then
				// there has been a change to the union dir. Return false.
				if !srvsMap[srvID] {
					return false
				}
			}
			// If the lengths are the same, and all services in urpcc.srvs are in
			// srvsMap, then the set of services in the union dir has not changed.
			return true
		})
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error ReadDirWatch monitorSrvs[%v]: %v", urpcc.path, err)
		}
		srvs := sp.Names(sts)
		db.DPrintf(urpcc.lSelector, "monitorSrvs new srv list: %v", srvs)
		// Update the list of servers.
		urpcc.Lock()
		urpcc.updateSrvsL(srvs)
		urpcc.Unlock()
	}
}
