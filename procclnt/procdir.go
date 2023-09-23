package procclnt

import (
	"fmt"
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
	childrenDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.MkDir(childrenDir, 0777); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "MakeProcDir mkdir childrens %v err %v\n", childrenDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
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

// Initialize a proc dir for this proc.
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
		}
		// Demote to reader lock.
		clnt.Unlock()
		clnt.RLock()
	}
	return err
}

// ========== HELPERS ==========

// Clean up proc
func removeProc(fsl *fslib.FsLib, procdir string) error {
	// Children may try to write in symlinks & exit statuses while the rmdir is
	// happening. In order to avoid causing errors (such as removing a non-empty
	// dir) temporarily rename so children can't find the dir. The dir may be
	// missing already if a proc died while exiting, and this is a procd trying
	// to exit on its behalf.
	src := path.Join(procdir, proc.CHILDREN)
	dst := path.Join(procdir, ".tmp."+proc.CHILDREN)
	if err := fsl.Rename(src, dst); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error rename removeProc %v -> %v : %v\n", src, dst, err)
	}
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

// Attempt to cleanup procdir
func (clnt *ProcClnt) cleanupError(pid sp.Tpid, procdir string, err error) error {
	clnt.RemoveChild(pid)
	// May be called by spawning parent proc, without knowing what the procdir is
	// yet.
	if len(procdir) > 0 {
		removeProc(clnt.FsLib, procdir)
	}
	return err
}

// ========== CHILDREN ==========

// Return the pids of all children.
func (clnt *ProcClnt) GetChildren() ([]sp.Tpid, error) {
	sts, err := clnt.GetDir(path.Join(proc.PROCDIR, proc.CHILDREN))
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "GetChildren %v error: %v", clnt.ProcEnv().ProcDir, err)
		return nil, err
	}
	cpids := []sp.Tpid{}
	for _, st := range sts {
		cpids = append(cpids, sp.Tpid(st.Name))
	}
	return cpids, nil
}

// Add a child to the current proc
func (clnt *ProcClnt) addChild(p *proc.Proc, childProcdir string, how proc.Thow) error {
	// Make sure this proc's ProcDir has been initialized. We to do this here so
	// that it can happen lazily (only for procs which spawn children).
	if err := clnt.initProcDir(); err != nil {
		db.DPrintf(db.ALWAYS, "Error init proc dir: %v", err)
		return err
	}
	// Directory which holds link to child procdir
	childDir := path.Dir(proc.GetChildProcDir(proc.PROCDIR, p.GetPid()))
	if err := clnt.MkDir(childDir, 0777); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Spawn mkdir childs %v err %v fsl %v", childDir, err, clnt.FsLib)
		return clnt.cleanupError(p.GetPid(), childProcdir, fmt.Errorf("Spawn error %v", err))
	}
	// Link in shared state from parent, if desired.
	if len(p.GetShared()) > 0 {
		if err := clnt.Symlink([]byte(p.GetShared()), path.Join(childDir, proc.SHARED), 0777); err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Error addChild Symlink: %v", err)
		}
	}
	return nil
}

// Remove a child from the current proc
func (clnt *ProcClnt) RemoveChild(pid sp.Tpid) error {
	childdir := path.Dir(proc.GetChildProcDir(proc.PROCDIR, pid))
	if err := clnt.RmDir(childdir); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error Remove 2 %v in RemoveChild: %v", childdir, err)
		return fmt.Errorf("RemoveChild dir error %v", err)
	}
	return nil
}
