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
		db.DLPrintf("PROCCLNT_ERR", "MakeProcDir mkdir pid %v err %v\n", procdir, err)
		return err
	}
	childrenDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.MkDir(childrenDir, 0777); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "MakeProcDir mkdir childrens %v err %v\n", childrenDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	if isKernelProc {
		kprocFPath := path.Join(procdir, proc.KERNEL_PROC)
		if _, err := clnt.PutFile(kprocFPath, 0777, np.OWRITE, []byte{}); err != nil {
			db.DLPrintf("PROCCLNT_ERR", "MakeProcDir MakeFile %v err %v", kprocFPath, err)
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

func (clnt *ProcClnt) linkChildIntoParentDir(pid proc.Tpid, procdir string) error {
	// Add symlink to child
	link := path.Join(proc.PARENTDIR, proc.PROCDIR)
	// May return file not found if parent exited.
	if err := clnt.Symlink([]byte(proc.GetProcDir()), link, 0777); err != nil && !np.IsErrUnavailable(err) {
		db.DLPrintf("PROCCLNT_ERR", "Spawn Symlink child %v err %v\n", link, err)
		return clnt.cleanupError(pid, procdir, err)
	}
	return nil
}

// ========== HELPERS ==========

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

// Add a child to the current proc
func (clnt *ProcClnt) addChild(pid proc.Tpid, procdir, shared string) error {
	// Directory which holds link to child procdir
	childDir := path.Dir(proc.GetChildProcDir(pid))
	if err := clnt.MkDir(childDir, 0777); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Spawn mkdir childs %v err %v\n", childDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	// Link in shared state from parent, if desired.
	if len(shared) > 0 {
		if err := clnt.Symlink([]byte(shared), path.Join(childDir, proc.SHARED), 0777); err != nil {
			db.DLPrintf("PROCCLNT_ERR", "Error addChild Symlink: %v", err)
		}
	}
	return nil
}

// Remove a child from the current proc
func (clnt *ProcClnt) removeChild(pid proc.Tpid) error {
	procdir := proc.GetChildProcDir(pid)
	childdir := path.Dir(procdir)
	// Remove link.
	if err := clnt.Remove(procdir); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild link error %v", err)
	}

	if err := clnt.RmDir(childdir); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild dir error %v", err)
	}
	return nil
}
