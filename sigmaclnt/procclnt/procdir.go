package procclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/proc"
	"sigmaos/util/coordination/barrier"
	sp "sigmaos/sigmap"
)

// For documentation on dir structure, see sigmaos/proc/dir.go
func (clnt *ProcClnt) MakeProcDir(pid sp.Tpid, procdir string, isKernelProc bool, how proc.Thow) error {
	if err := clnt.MkDir(procdir, 0777); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "MakeProcDir mkdir pid %v procdir %v err %v\n", pid, procdir, err)
		return err
	}
	// Only create exit/evict semaphores if not spawned on MSCHED.
	if how != proc.HMSCHED {
		// Create exit signal
		semExit := barrier.NewBarrier(clnt.FsLib, filepath.Join(procdir, proc.EXIT_SEM))
		semExit.Init(0)

		// Create eviction signal
		semEvict := barrier.NewBarrier(clnt.FsLib, filepath.Join(procdir, proc.EVICT_SEM))
		semEvict.Init(0)
	}
	return nil
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
