package procclnt

import (
	"fmt"
	"log"
	"path"

	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/semclnt"
)

// For documentation on dir structure, see ulambda/proc/dir.go

func (clnt *ProcClnt) MakeProcDir(pid, procdir string, isKernelProc bool) error {
	if err := clnt.Mkdir(procdir, 0777); err != nil {
		log.Printf("%v: MakeProcDir mkdir pid %v err %v\n", proc.GetProgram(), procdir, err)
		return err
	}
	childrenDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.Mkdir(childrenDir, 0777); err != nil {
		log.Printf("%v: MakeProcDir mkdir childrens %v err %v\n", proc.GetProgram(), childrenDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	if isKernelProc {
		kprocFPath := path.Join(procdir, proc.KERNEL_PROC)
		if err := clnt.MakeFile(kprocFPath, 0777, np.OWRITE, []byte{}); err != nil {
			log.Printf("%v: MakeProcDir MakeFile %v err %v", proc.GetProgram(), kprocFPath, err)
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

func (clnt *ProcClnt) linkChildIntoParentDir(pid, procdir string) error {
	// Add symlink to child
	link := path.Join(proc.PARENTDIR, proc.PROCDIR)
	// May return file not found if parent exited.
	if err := clnt.Symlink([]byte(proc.GetProcDir()), link, 0777); err != nil && err.Error() != "file not found" {
		log.Printf("%v: Spawn Symlink child %v err %v\n", proc.GetProgram(), link, err)
		return clnt.cleanupError(pid, procdir, err)
	}
	return nil
}

// ========== HELPERS ==========

// Attempt to cleanup procdir
func (clnt *ProcClnt) cleanupError(pid, procdir string, err error) error {
	clnt.removeChild(pid)
	// May be called by spawning parent proc, without knowing what the procdir is
	// yet.
	if len(procdir) > 0 {
		clnt.removeProc(procdir)
	}
	return err
}

func (clnt *ProcClnt) isKernelProc(pid string) bool {
	_, err := clnt.Stat(path.Join(proc.PROCDIR, proc.KERNEL_PROC))
	return err == nil
}

// ========== CHILDREN ==========

// Add a child to the current proc
func (clnt *ProcClnt) addChild(pid, procdir, shared string) error {
	// Directory which holds link to child procdir
	childDir := path.Dir(proc.GetChildProcDir(pid))
	if err := clnt.Mkdir(childDir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir childs %v err %v\n", proc.GetProgram(), childDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	// Link in shared state from parent, if desired.
	if len(shared) > 0 {
		if err := clnt.Symlink([]byte(shared), path.Join(childDir, proc.SHARED), 0777); err != nil {
			log.Printf("%v: Error addChild Symlink: %v", proc.GetProgram(), err)
		}
	}
	return nil
}

// Remove a child from the current proc
func (clnt *ProcClnt) removeChild(pid string) error {
	procdir := proc.GetChildProcDir(pid)
	childdir := path.Dir(procdir)
	// Remove link.
	if err := clnt.Remove(procdir); err != nil {
		log.Printf("Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild link error %v", err)
	}

	if err := clnt.RmDir(childdir); err != nil {
		log.Printf("Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild dir error %v", err)
	}
	return nil
}
