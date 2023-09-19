package procqclnt

import (
	"errors"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procqsrv/proto"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	NOT_ENQ = "NOT_ENQUEUED"
)

type ProcQClnt struct {
	done int32
	*fslib.FsLib
	sync.Mutex
	procqs      map[string]*rpcclnt.RPCClnt
	procqIDs    []string
	lastUpdate  time.Time
	burstOffset int
}

func NewProcQClnt(fsl *fslib.FsLib) *ProcQClnt {
	return &ProcQClnt{
		FsLib:    fsl,
		procqs:   make(map[string]*rpcclnt.RPCClnt),
		procqIDs: make([]string, 0),
	}
}

func (pqc *ProcQClnt) Nprocq() (int, error) {
	sds, err := pqc.getProcQs()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
}

// Enqueue a proc on the procq. Returns the ID of the kernel that is running
// the proc.
func (pqc *ProcQClnt) Enqueue(p *proc.Proc) (string, error) {
	pqc.UpdateProcQs()
	pqID, err := pqc.NextProcQ()
	if err != nil {
		return NOT_ENQ, errors.New("No procqs available")
	}
	rpcc, err := pqc.GetProcQClnt(pqID)
	if err != nil {
		db.DFatalf("Error: Can't get procq clnt: %v", err)
		return NOT_ENQ, err
	}
	req := &proto.EnqueueRequest{
		ProcProto: p.GetProto(),
	}
	res := &proto.EnqueueResponse{}
	if err := rpcc.RPC("ProcQ.Enqueue", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "ProcQ.Enqueue err %v", err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
			pqc.UnregisterClnt(pqID)
		}
		return NOT_ENQ, err
	}
	db.DPrintf(db.PROCQCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
	return res.KernelID, nil
}

// Get a proc (passing in the kernelID of the caller). Will only return once
// successful, or once there is an error.
func (pqc *ProcQClnt) GetProc(callerKernelID string) (*proc.Proc, error) {
	// TODO: seems a bit sketchy to loop infinitely.
	// Retry until successful.
	pqc.UpdateProcQs()
	for {
		pqID, err := pqc.NextProcQ()
		if err != nil {
			pqc.UpdateProcQs()
			db.DPrintf(db.PROCQCLNT_ERR, "No procQs available: %v", err)
			continue
		}
		rpcc, err := pqc.GetProcQClnt(pqID)
		if err != nil {
			db.DPrintf(db.PROCQCLNT_ERR, "Error: Can't get procq clnt: %v", err)
			return nil, err
		}
		req := &proto.GetProcRequest{
			KernelID: callerKernelID,
		}
		res := &proto.GetProcResponse{}
		if err := rpcc.RPC("ProcQ.GetProc", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "ProcQ.GetProc %v err %v", callerKernelID, err)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Force lookup %v", pqID)
				pqc.UnregisterClnt(pqID)
				continue
			}
			return nil, err
		}
		p := proc.NewProcFromProto(res.ProcProto)
		db.DPrintf(db.PROCQCLNT, "[%v] Got Proc %v", p.GetRealm(), p)
		return p, nil
	}
}

func (pqc *ProcQClnt) GetProcQClnt(procqID string) (*rpcclnt.RPCClnt, error) {
	pqc.Lock()
	defer pqc.Unlock()

	var rpcc *rpcclnt.RPCClnt
	var ok bool
	if rpcc, ok = pqc.procqs[procqID]; !ok {
		var err error
		rpcc, err = rpcclnt.NewRPCClnt([]*fslib.FsLib{pqc.FsLib}, path.Join(sp.PROCQ, procqID))
		if err != nil {
			db.DPrintf(db.PROCQCLNT_ERR, "Error newRPCClnt[procq:%v]: %v", procqID, err)
			return nil, err
		}
		pqc.procqs[procqID] = rpcc
	}
	return rpcc, nil
}

// Update the list of active procds.
func (pqc *ProcQClnt) UpdateProcQs() {
	pqc.Lock()
	defer pqc.Unlock()

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if time.Since(pqc.lastUpdate) < sp.Conf.Realm.RESIZE_INTERVAL && len(pqc.procqIDs) > 0 {
		db.DPrintf(db.PROCQCLNT, "Update procqs too soon")
		return
	}
	// Read the procd union dir.
	procqs, err := pqc.getProcQs()
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error ReadDir procd: %v", err)
		return
	}
	pqc.lastUpdate = time.Now()
	db.DPrintf(db.PROCQCLNT, "Got procqs %v", procqs)
	// Alloc enough space for the list of procqs.
	pqc.procqIDs = make([]string, 0, len(procqs))
	for _, procq := range procqs {
		pqc.procqIDs = append(pqc.procqIDs, procq)
	}
}

func (pqc *ProcQClnt) UnregisterClnt(id string) {
	pqc.Lock()
	defer pqc.Unlock()

	delete(pqc.procqs, id)
}

func (pqc *ProcQClnt) getProcQs() ([]string, error) {
	sts, err := pqc.GetDir(sp.PROCQ)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

// Get the next procd to burst on.
func (pqc *ProcQClnt) NextProcQ() (string, error) {
	pqc.Lock()
	defer pqc.Unlock()

	if len(pqc.procqIDs) == 0 {
		return "", serr.NewErr(serr.TErrNotfound, "no procqs to spawn on")
	}

	sdip := pqc.procqIDs[pqc.burstOffset%len(pqc.procqIDs)]
	pqc.burstOffset++
	return sdip, nil
}
