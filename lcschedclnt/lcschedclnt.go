package lcschedclnt

import (
	"errors"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	//	"sigmaos/lcschedsrv/proto"
	"sigmaos/proc"
	pqproto "sigmaos/procqsrv/proto"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type LCSchedClnt struct {
	done int32
	*fslib.FsLib
	mu          sync.Mutex
	lcscheds    map[string]*rpcclnt.RPCClnt
	lcschedIDs  []string
	lastUpdate  time.Time
	burstOffset int
}

func NewLCSchedClnt(fsl *fslib.FsLib) *LCSchedClnt {
	return &LCSchedClnt{
		FsLib:      fsl,
		lcscheds:   make(map[string]*rpcclnt.RPCClnt),
		lcschedIDs: make([]string, 0),
	}
}

func (lcs *LCSchedClnt) Nlcsched() (int, error) {
	sds, err := lcs.getLCScheds()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
}

// Enqueue a proc on the lcsched. Returns the ID of the kernel that is running
// the proc.
func (lcs *LCSchedClnt) Enqueue(p *proc.Proc) (string, error) {
	lcs.UpdateLCScheds()
	pqID, err := lcs.NextLCSched()
	if err != nil {
		return NOT_ENQ, errors.New("No lcscheds available")
	}
	rpcc, err := lcs.GetLCSchedClnt(pqID)
	if err != nil {
		db.DFatalf("Error: Can't get lcsched clnt: %v", err)
		return NOT_ENQ, err
	}
	req := &pqproto.EnqueueRequest{
		ProcProto: p.GetProto(),
	}
	res := &pqproto.EnqueueResponse{}
	if err := rpcc.RPC("LCSched.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "LCSched.Enqueue err %v", err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
			lcs.UnregisterClnt(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.LCSCHEDCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.KernelID, nil
}

func (lcs *LCSchedClnt) GetLCSchedClnt(lcschedID string) (*rpcclnt.RPCClnt, error) {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	var rpcc *rpcclnt.RPCClnt
	var ok bool
	if rpcc, ok = lcs.lcscheds[lcschedID]; !ok {
		var err error
		rpcc, err = rpcclnt.NewRPCClnt([]*fslib.FsLib{lcs.FsLib}, path.Join(sp.LCSCHED, lcschedID))
		if err != nil {
			db.DPrintf(db.LCSCHEDCLNT_ERR, "Error newRPCClnt[lcsched:%v]: %v", lcschedID, err)
			return nil, err
		}
		lcs.lcscheds[lcschedID] = rpcc
	}
	return rpcc, nil
}

// Update the list of active procds.
func (lcs *LCSchedClnt) UpdateLCScheds() {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if time.Since(lcs.lastUpdate) < sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL && len(lcs.lcschedIDs) > 0 {
		db.DPrintf(db.LCSCHEDCLNT, "Update lcscheds too soon")
		return
	}
	// Read the procd union dir.
	lcscheds, err := lcs.getLCScheds()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error ReadDir procd: %v", err)
		return
	}
	lcs.lastUpdate = time.Now()
	db.DPrintf(db.LCSCHEDCLNT, "Got lcscheds %v", lcscheds)
	// Alloc enough space for the list of lcscheds.
	lcs.lcschedIDs = make([]string, 0, len(lcscheds))
	for _, lcsched := range lcscheds {
		lcs.lcschedIDs = append(lcs.lcschedIDs, lcsched)
	}
}

func (lcs *LCSchedClnt) UnregisterClnt(id string) {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	delete(lcs.lcscheds, id)
}

func (lcs *LCSchedClnt) getLCScheds() ([]string, error) {
	sts, err := lcs.GetDir(sp.LCSCHED)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

// Get the next procd to burst on.
func (lcs *LCSchedClnt) NextLCSched() (string, error) {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	if len(lcs.lcschedIDs) == 0 {
		return "", serr.NewErr(serr.TErrNotfound, "no lcscheds to spawn on")
	}

	sdip := lcs.lcschedIDs[lcs.burstOffset%len(lcs.lcschedIDs)]
	lcs.burstOffset++
	return sdip, nil
}
