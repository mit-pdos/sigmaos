package unionrpcclnt

import (
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type UnionRPCClnt struct {
	*fslib.FsLib
	sync.Mutex
	path       string
	clnts      map[string]*rpcclnt.RPCClnt
	srvs       []string
	lastUpdate time.Time
	rrOffset   int
	lSelector  db.Tselector
	eSelector  db.Tselector
}

func NewUnionRPCClnt(fsl *fslib.FsLib, path string, lSelector db.Tselector, eSelector db.Tselector) *UnionRPCClnt {
	return &UnionRPCClnt{
		FsLib: fsl,
		path:  path,
		clnts: make(map[string]*rpcclnt.RPCClnt),
		srvs:  make([]string, 0),
	}
}

func (urpcc *UnionRPCClnt) Nsrv() (int, error) {
	sds, err := urpcc.getSrvs()
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

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if force || time.Since(urpcc.lastUpdate) < sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL && len(urpcc.srvs) > 0 {
		db.DPrintf(urpcc.lSelector, "Update clnts too soon")
		return
	}
	// Read the procd union dir.
	clnts, err := urpcc.getSrvs()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error ReadDir procd: %v", err)
		return
	}
	urpcc.lastUpdate = time.Now()
	db.DPrintf(urpcc.lSelector, "Got clnts %v", clnts)
	// Alloc enough space for the list of clnts.
	urpcc.srvs = make([]string, 0, len(clnts))
	for _, procq := range clnts {
		urpcc.srvs = append(urpcc.srvs, procq)
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

func (urpcc *UnionRPCClnt) getSrvs() ([]string, error) {
	sts, err := urpcc.GetDir(urpcc.path)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
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
