package procclnt

import (
	"path"
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

// For documentation on dir structure, see sigmaos/proc/dir.go

func (clnt *ProcClnt) MakeProcDir(pid sp.Tpid, procdir string, isKernelProc bool, how proc.Thow) error {
	if err := clnt.MkDir(procdir, 0777); err != nil {
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			debug.PrintStack()
			db.DFatalf("MakeProcDir mkdir pid %v procdir %v err %v\n", pid, procdir, err)
		}
		db.DPrintf(db.PROCCLNT_ERR, "MakeProcDir mkdir pid %v procdir %v err %v\n", pid, procdir, err)
		return err
	}
	// Only create exit/evict semaphores if not spawned on SCHEDD.
	if how != proc.HSCHEDD {
		// Create exit signal
		semExit := semclnt.NewSemClnt(clnt.FsLib, path.Join(procdir, proc.EXIT_SEM))
		semExit.Init(0)

		// Create eviction signal
		semEvict := semclnt.NewSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
		semEvict.Init(0)
	}
	return nil
}

// Initialize a proc dir for this proc (but make sure to only do so once).
func (clnt *ProcClnt) initProcDir() error {
	clnt.RLock()
	defer clnt.RUnlock()

	var err error
	if !clnt.procDirCreated {
		// Promote to writer lock.
		clnt.RUnlock()
		clnt.Lock()
		// Check that the proc dir still has not been created after lock promotion.
		if !clnt.procDirCreated {
			// Make a ProcDir for this proc.
			err = clnt.MakeProcDir(clnt.ProcEnv().GetPID(), clnt.ProcEnv().GetProcDir(), clnt.ProcEnv().GetPrivileged(), clnt.ProcEnv().GetHow())
			clnt.procDirCreated = true
			// Mount procdir
			db.DPrintf(db.PROCCLNT, "Mount %v as %v", clnt.ProcEnv().ProcDir, proc.PROCDIR)
			clnt.NewRootMount(clnt.ProcEnv().GetUname(), clnt.ProcEnv().ProcDir, proc.PROCDIR)
		}
		// Demote to reader lock.
		clnt.Unlock()
		clnt.RLock()
	}
	return err
}

// ========== HELPERS ==========

// Clean up proc
func removeProc(fsl *fslib.FsLib, procdir string, createdProcDir bool) error {
	if createdProcDir {
		err := fsl.RmDir(procdir)
		maxRetries := 2
		// May have to retry a few times if writing child already opened dir. We
		// should only have to retry once at most.
		for i := 0; i < maxRetries && err != nil; i++ {
			s, _ := fsl.SprintfDir(procdir)
			// debug.PrintStack()
			db.DPrintf(db.PROCCLNT_ERR, "RmDir %v err %v \n%v", procdir, err, s)
			// Retry
			err = fsl.RmDir(procdir)
		}
		return err
	}
	return nil
}

// Attempt to cleanup procdir
func (clnt *ProcClnt) cleanupError(pid sp.Tpid, procdir string, err error) error {
	// May be called by spawning parent proc, without knowing what the procdir is
	// yet.
	if len(procdir) > 0 {
		removeProc(clnt.FsLib, procdir, true)
	}
	return err
}
