package procmgr

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocclnt"
)

type ProcMgr struct {
	sync.Mutex
	rootsc  *sigmaclnt.SigmaClnt
	updm    *uprocclnt.UprocdMgr
	sclnts  map[sp.Trealm]*sigmaclnt.SigmaClnt
	running map[proc.Tpid]*proc.Proc
}

// Manages the state and lifecycle of a proc.
func MakeProcMgr(rootsc *sigmaclnt.SigmaClnt) *ProcMgr {
	return &ProcMgr{
		rootsc:  rootsc,
		updm:    uprocclnt.MakeUprocdMgr(rootsc.FsLib),
		sclnts:  make(map[sp.Trealm]*sigmaclnt.SigmaClnt),
		running: make(map[proc.Tpid]*proc.Proc),
	}
}

func (mgr *ProcMgr) RunProc(p *proc.Proc) {
	mgr.setupProcState(p)
	mgr.downloadProc(p)
	mgr.runProc(p)
	mgr.teardownProcState(p)
}

func (mgr *ProcMgr) getSigmaClnt(realm sp.Trealm) *sigmaclnt.SigmaClnt {
	mgr.Lock()
	defer mgr.Unlock()

	var clnt *sigmaclnt.SigmaClnt
	var ok bool
	if clnt, ok = mgr.sclnts[realm]; !ok {
		// No need to make a new client for the root realm.
		if realm == sp.Trealm(proc.GetRealm()) {
			clnt = &sigmaclnt.SigmaClnt{mgr.rootsc.FsLib, nil}
		} else {
			var err error
			if clnt, err = sigmaclnt.MkSigmaClntRealm(mgr.rootsc.FsLib, sp.SCHEDDREL, realm); err != nil {
				db.DFatalf("Err MkSigmaClntRealm: %v", err)
			}
			// Mount KPIDS.
			procclnt.MountPids(clnt.FsLib, clnt.FsLib.NamedAddr())
		}
		mgr.sclnts[realm] = clnt
	}
	return clnt
}
