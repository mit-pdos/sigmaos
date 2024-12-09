package procclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	leaseclnt "sigmaos/ft/lease/clnt"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

// For documentation on dir structure, see sigmaos/proc/dir.go
func (clnt *ProcClnt) MakeProcDir(pid sp.Tpid, procdir string) error {
	if err := clnt.MkDir(procdir, 0777); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "MakeProcDir mkdir pid %v procdir %v err %v\n", pid, procdir, err)
		return err
	}
	return nil
}

func MakeKProcSemaphores(fsl *fslib.FsLib, lc *leaseclnt.LeaseClnt) error {
	db.DPrintf(db.PROCCLNT, "MakeKProcSemaphores")
	exitSemPN := filepath.Join(fsl.ProcEnv().GetProcDir(), proc.EXIT_SEM)
	evictSemPN := filepath.Join(fsl.ProcEnv().GetProcDir(), proc.EVICT_SEM)
	li, err := lc.AskLease(exitSemPN, fsetcd.LeaseTTL)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Err AskLease: %v", err)
	}
	// Create exit signal
	semExit := semclnt.NewSemClnt(fsl, exitSemPN)
	if err := semExit.InitLease(0777, li.Lease()); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Err init lease sem [%v]: %v", exitSemPN, err)
	}
	// Create eviction signal
	semEvict := semclnt.NewSemClnt(fsl, evictSemPN)
	if err := semEvict.InitLease(0777, li.Lease()); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Err init lease sem [%v]: %v", evictSemPN, err)
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
