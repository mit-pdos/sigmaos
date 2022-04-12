package procclnt

import (
	"fmt"
	"path"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/semclnt"
)

// For documentation on dir structure, see ulambda/proc/dir.go

func (clnt *ProcClnt) MakeProcDir(pid proc.Tpid, procdir string, isKernelProc bool) error {
	if err := clnt.MkDir(procdir, 0777); err != nil {
		db.DPrintf("PROCCLNT_ERR", "MakeProcDir mkdir pid %v err %v\n", procdir, err)
		return err
	}
	childrenDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.MkDir(childrenDir, 0777); err != nil {
		db.DPrintf("PROCCLNT_ERR", "MakeProcDir mkdir childrens %v err %v\n", childrenDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	if isKernelProc {
		kprocFPath := path.Join(procdir, proc.KERNEL_PROC)
		if _, err := clnt.PutFile(kprocFPath, 0777, np.OWRITE, []byte{}); err != nil {
			db.DPrintf("PROCCLNT_ERR", "MakeProcDir MakeFile %v err %v", kprocFPath, err)
			return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
	}

	// Create exit signal
	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EXIT_SEM))
	semExit.Init(0)

	// Create eviction signal
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	semEvict.Init(0)

	return nil
}

// ========== SYMLINKS ==========

func (clnt *ProcClnt) linkSelfIntoParentDir() error {
	// Add symlink to child
	link := path.Join(proc.PARENTDIR, proc.PROCDIR)
	// Find the procdir. For normally-spawned procs, this will be proc.PROCDIR,
	// which is just a mount point. Rather, the Symlink will need the full path
	// for the parent to mount the child directory.
	var procdir string
	if clnt.procdir == proc.PROCDIR {
		procdir = proc.GetProcDir()
	} else {
		procdir = clnt.procdir
	}
	// May return file not found if parent exited.
	if err := clnt.Symlink([]byte(procdir), link, 0777); err != nil && !np.IsErrUnavailable(err) {
		db.DPrintf("PROCCLNT_ERR", "Spawn Symlink child %v err %v\n", link, err)
		return clnt.cleanupError(clnt.pid, clnt.procdir, err)
	}
	return nil
}

// ========== HELPERS ==========

// Clean up proc
func (clnt *ProcClnt) removeProc(procdir string) error {
	// Children may try to write in symlinks & exit statuses while the rmdir is
	// happening. In order to avoid causing errors (such as removing a non-empty
	// dir) temporarily rename so children can't find the dir. The dir may be
	// missing already if a proc died while exiting, and this is a procd trying
	// to exit on its behalf.
	src := path.Join(procdir, proc.CHILDREN)
	dst := path.Join(procdir, ".tmp."+proc.CHILDREN)
	if err := clnt.Rename(src, dst); err != nil {
		db.DPrintf("PROCCLNT_ERR", "Error rename removeProc %v -> %v : %v\n", src, dst, err)
	}
	err := clnt.RmDir(procdir)
	maxRetries := 2
	// May have to retry a few times if writing child already opened dir. We
	// should only have to retry once at most.
	for i := 0; i < maxRetries && err != nil; i++ {
		s, _ := clnt.SprintfDir(procdir)
		// debug.PrintStack()
		db.DPrintf("PROCCLNT_ERR", "RmDir %v err %v \n%v", procdir, err, s)
		// Retry
		err = clnt.RmDir(procdir)
	}
	return err
}

// Attempt to cleanup procdir
func (clnt *ProcClnt) cleanupError(pid proc.Tpid, procdir string, err error) error {
	clnt.removeChild(pid)
	// May be called by spawning parent proc, without knowing what the procdir is
	// yet.
	if len(procdir) > 0 {
		clnt.removeProc(procdir)
	}
	return err
}

func (clnt *ProcClnt) isKernelProc(pid proc.Tpid) bool {
	_, err := clnt.Stat(path.Join(proc.PROCDIR, proc.KERNEL_PROC))
	return err == nil
}

// ========== CHILDREN ==========

// Return the pids of all children.
func (clnt *ProcClnt) GetChildren() ([]proc.Tpid, error) {
	sts, err := clnt.GetDir(path.Join(clnt.procdir, proc.CHILDREN))
	if err != nil {
		db.DPrintf("PROCCLNT_ERR", "GetChildren %v error: %v", clnt.procdir, err)
		return nil, err
	}
	cpids := []proc.Tpid{}
	for _, st := range sts {
		cpids = append(cpids, proc.Tpid(st.Name))
	}
	return cpids, nil
}

// Add a child to the current proc
func (clnt *ProcClnt) addChild(pid proc.Tpid, childProcdir, shared string) error {
	// Directory which holds link to child procdir
	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
	if err := clnt.MkDir(childDir, 0777); err != nil {
		db.DPrintf("PROCCLNT_ERR", "Spawn mkdir childs %v err %v\n", childDir, err)
		return clnt.cleanupError(pid, childProcdir, fmt.Errorf("Spawn error %v", err))
	}
	// Link in shared state from parent, if desired.
	if len(shared) > 0 {
		if err := clnt.Symlink([]byte(shared), path.Join(childDir, proc.SHARED), 0777); err != nil {
			db.DPrintf("PROCCLNT_ERR", "Error addChild Symlink: %v", err)
		}
	}
	return nil
}

// Remove a child from the current proc
func (clnt *ProcClnt) removeChild(pid proc.Tpid) error {
	procdir := proc.GetChildProcDir(clnt.procdir, pid)
	childdir := path.Dir(procdir)
	// Remove link.
	if err := clnt.Remove(procdir); err != nil {
		db.DPrintf("PROCCLNT_ERR", "Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild link error %v", err)
	}

	if err := clnt.RmDir(childdir); err != nil {
		db.DPrintf("PROCCLNT_ERR", "Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild dir error %v", err)
	}
	return nil
}
