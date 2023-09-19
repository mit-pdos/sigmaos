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
	sync.Mutex
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

func (pqc *LCSchedClnt) Nlcsched() (int, error) {
	sds, err := pqc.getLCScheds()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
}

// Enqueue a proc on the lcsched. Returns the ID of the kernel that is running
// the proc.
func (pqc *LCSchedClnt) Enqueue(p *proc.Proc) (string, error) {
	pqc.UpdateLCScheds()
	pqID, err := pqc.NextLCSched()
	if err != nil {
		return NOT_ENQ, errors.New("No lcscheds available")
	}
	rpcc, err := pqc.GetLCSchedClnt(pqID)
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
			pqc.UnregisterClnt(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.LCSCHEDCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.KernelID, nil
}

func (pqc *LCSchedClnt) GetLCSchedClnt(lcschedID string) (*rpcclnt.RPCClnt, error) {
	pqc.Lock()
	defer pqc.Unlock()

	var rpcc *rpcclnt.RPCClnt
	var ok bool
	if rpcc, ok = pqc.lcscheds[lcschedID]; !ok {
		var err error
		rpcc, err = rpcclnt.NewRPCClnt([]*fslib.FsLib{pqc.FsLib}, path.Join(sp.LCSCHED, lcschedID))
		if err != nil {
			db.DPrintf(db.LCSCHEDCLNT_ERR, "Error newRPCClnt[lcsched:%v]: %v", lcschedID, err)
			return nil, err
		}
		pqc.lcscheds[lcschedID] = rpcc
	}
	return rpcc, nil
}

// Update the list of active procds.
func (pqc *LCSchedClnt) UpdateLCScheds() {
	pqc.Lock()
	defer pqc.Unlock()

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if time.Since(pqc.lastUpdate) < sp.Conf.Realm.RESIZE_INTERVAL && len(pqc.lcschedIDs) > 0 {
		db.DPrintf(db.LCSCHEDCLNT, "Update lcscheds too soon")
		return
	}
	// Read the procd union dir.
	lcscheds, err := pqc.getLCScheds()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error ReadDir procd: %v", err)
		return
	}
	pqc.lastUpdate = time.Now()
	db.DPrintf(db.LCSCHEDCLNT, "Got lcscheds %v", lcscheds)
	// Alloc enough space for the list of lcscheds.
	pqc.lcschedIDs = make([]string, 0, len(lcscheds))
	for _, lcsched := range lcscheds {
		pqc.lcschedIDs = append(pqc.lcschedIDs, lcsched)
	}
}

func (pqc *LCSchedClnt) UnregisterClnt(id string) {
	pqc.Lock()
	defer pqc.Unlock()

	delete(pqc.lcscheds, id)
}

func (pqc *LCSchedClnt) getLCScheds() ([]string, error) {
	sts, err := pqc.GetDir(sp.LCSCHED)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

// Get the next procd to burst on.
func (pqc *LCSchedClnt) NextLCSched() (string, error) {
	pqc.Lock()
	defer pqc.Unlock()

	if len(pqc.lcschedIDs) == 0 {
		return "", serr.NewErr(serr.TErrNotfound, "no lcscheds to spawn on")
	}

	sdip := pqc.lcschedIDs[pqc.burstOffset%len(pqc.lcschedIDs)]
	pqc.burstOffset++
	return sdip, nil
}
