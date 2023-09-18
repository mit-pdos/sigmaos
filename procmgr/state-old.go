package procmgr

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

//
// Proc state management in the realm namespace.
//

// Post a proc file in the local queue.
func (mgr *ProcMgr) postProcInQueue(p *proc.Proc) {
	if _, err := mgr.mfs.Create(path.Join(sp.QUEUE, p.GetPid().String()), 0777, sp.OWRITE, sp.NoLeaseId); err != nil {
		db.DFatalf("Error create %v: %v", p.GetPid(), err)
	}
}

// Create an ephemeral "Started" semaphore. Must be ephemeral so parent procs can detect schedd crashes.
func (mgr *ProcMgr) createStartedSem(p *proc.Proc) (*semclnt.SemClnt, error) {
	semPath := path.Join(p.GetParentDir(), proc.START_SEM)
	semStart := semclnt.NewSemClnt(mgr.getSigmaClnt(p.GetRealm()).FsLib, semPath)
	var err error
	if err = semStart.Init(sp.DMTMP); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Err sem init [%v]: %v", semPath, err)
	} else {
		db.DPrintf(db.PROCMGR, "Sem init done: %v", p)
	}
	return semStart, err
}

// Set up a proc's state in the realm.
func (mgr *ProcMgr) setupProcState(p *proc.Proc) {
	mgr.addRunningProc(p)
	// Set up the directory to cache proc binaries for this realm.
	mgr.setupUserBinCache(p)
	// Create started semaphore, if the proc was not stolen. If the proc was
	// stolen, the started semaphore would have been created as part of the
	// stealing process.
	if _, err := mgr.createStartedSem(p); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Error creating start semaphore path:%v err:%v", path.Join(p.GetParentDir(), proc.START_SEM), err)
	}
	// Release the parent proc, which may be waiting for removal of the proc
	// queue file in WaitStart.
	if err := mgr.rootsc.Remove(path.Join(sp.SCHEDD, p.GetKernelID(), sp.QUEUE, p.GetPid().String())); err != nil {
		// Check if the proc was stoelln from another schedd.
		stolen := p.GetKernelID() != mgr.kernelId
		if stolen {
			// May return an error if the schedd stolen from crashes.
			db.DPrintf(db.PROCMGR_ERR, "Error remove schedd queue file [%v]: %v", p.GetKernelID(), err)
		} else {
			// Removing from self should always succeed.
			db.DFatalf("Error remove schedd queue file: %v", err)
		}
	}
	// Make the proc's procdir
	if err := mgr.rootsc.NewProcDir(p.GetPid(), p.GetProcDir(), p.IsPrivileged()); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Err procmgr NewProcDir: %v\n", err)
	}
}

func (mgr *ProcMgr) teardownProcState(p *proc.Proc) {
	mgr.removeRunningProc(p)
}

// Set up state to notify parent that a proc crashed.
func (mgr *ProcMgr) procCrashed(p *proc.Proc, err error) {
	db.DPrintf(db.PROCMGR_ERR, "Proc %v finished with error: %v", p, err)
	mgr.pstate.started(p.GetPid())
	procclnt.ExitedProcd(mgr.getSigmaClnt(p.GetRealm()).FsLib, p.GetPid(), p.GetProcDir(), p.GetParentDir(), proc.NewStatusErr(err.Error(), nil), p.GetHow())
}

// Register a proc as running.
func (mgr *ProcMgr) addRunningProc(p *proc.Proc) {
	mgr.Lock()
	defer mgr.Unlock()

	// XXX Write package to expose running map as a dir.
	mgr.running[p.GetPid()] = p
	_, err := mgr.rootsc.PutFile(path.Join(sp.SCHEDD, mgr.kernelId, sp.RUNNING, p.GetPid().String()), 0777, sp.OREAD|sp.OWRITE, p.MarshalJson())
	if err != nil {
		db.DFatalf("Error PutFile in running queue: %v", err)
	}
}

// Unregister a proc which has finished running.
func (mgr *ProcMgr) removeRunningProc(p *proc.Proc) {
	mgr.Lock()
	defer mgr.Unlock()

	// XXX Write package to expose running map as a dir.
	delete(mgr.running, p.GetPid())
	if err := mgr.mfs.Remove(path.Join(sp.RUNNING, p.GetPid().String())); err != nil {
		db.DFatalf("Error Remove from running queue: %v", err)
	}
}

// ========== Work-stealing ==========

func getWSQueuePath(ptype proc.Ttype) string {
	var q string
	switch ptype {
	case proc.T_LC:
		q = sp.WS_RUNQ_LC
	case proc.T_BE:
		q = sp.WS_RUNQ_BE
	default:
		db.DFatalf("Unrecognized proc type: %v", ptype)
	}
	return q
}

func (mgr *ProcMgr) removeWSLink(p *proc.Proc) {
	mgr.rootsc.Remove(path.Join(getWSQueuePath(p.GetType()), p.GetPid().String()))
}

func (mgr *ProcMgr) createWSLink(p *proc.Proc) {
	if _, err := mgr.rootsc.PutFile(path.Join(getWSQueuePath(p.GetType()), p.GetPid().String()), 0777, sp.OWRITE, p.Marshal()); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
}

func (mgr *ProcMgr) getWSQueue(qpath string) (map[sp.Trealm][]*proc.Proc, bool) {
	stealable := make(map[sp.Trealm][]*proc.Proc, 0)
	// Wait until there is a proc to steal.
	sts, err := mgr.rootsc.ReadDirWatch(qpath, func(sts []*sp.Stat) bool {
		var nStealable int
		for _, st := range sts {
			var p *proc.Proc
			var ok bool
			// Try to tread the proc from the cache.
			if p, ok = mgr.pcache.Get(sp.Tpid(st.Name)); !ok {
				// Read and unmarshal proc.
				b, err := mgr.rootsc.GetFile(path.Join(qpath, st.Name))
				if err != nil {
					// Proc may have been stolen already.
					continue
				}
				p = proc.NewEmptyProc()
				p.Unmarshal(b)
				mgr.pcache.Set(p.GetPid(), p)
			}
			// Is the proc a local proc? If so, don't add it to the queue of
			// stealable procs.
			if p.GetKernelID() == mgr.kernelId {
				continue
			}
			if _, ok := stealable[p.GetRealm()]; !ok {
				stealable[p.GetRealm()] = make([]*proc.Proc, 0)
			}
			// Add to the list of stealable procs
			stealable[p.GetRealm()] = append(stealable[p.GetRealm()], p)
			nStealable++
		}
		db.DPrintf(db.PROCMGR, "Found %v stealable procs %v", nStealable, stealable)
		return nStealable == 0
	})
	// Since many schedds may be modifying the WS dir, we may get version
	// errors.
	if serr.IsErrCode(err, serr.TErrVersion) {
		db.DPrintf(db.PROCMGR_ERR, "Error ReadDirWatch: %v %v", err, len(sts))
		return nil, false
	}
	if serr.IsErrCode(err, serr.TErrUnreachable) {
		db.DPrintf(db.PROCMGR_ERR, "Error ReadDirWatch: %v %v", err, len(sts))
		return nil, false
	}
	if err != nil {
		db.DFatalf("Error ReadDirWatch: %v %v", err, len(sts))
	}
	return stealable, true
}
