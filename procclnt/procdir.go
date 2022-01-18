package procclnt

import (
	"fmt"
	"log"
	"path"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
)

// For documentation on dir structure, see ulambda/proc/dir.go

func (clnt *ProcClnt) MakeProcDir(pid, procdir string, isKernelProc bool) error {
	if err := clnt.Mkdir(procdir, 0777); err != nil {
		log.Printf("%v: MakeProcDir mkdir pid %v err %v\n", db.GetName(), procdir, err)
		return err
	}
	childrenDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.Mkdir(childrenDir, 0777); err != nil {
		log.Printf("%v: MakeProcDir mkdir childrens %v err %v\n", db.GetName(), childrenDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	if isKernelProc {
		kprocFPath := path.Join(procdir, proc.KERNEL_PROC)
		if err := clnt.MakeFile(kprocFPath, 0777, np.OWRITE, []byte{}); err != nil {
			log.Printf("%v: MakeProcDir MakeFile %v err %v", db.GetName(), kprocFPath, err)
			return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
	}
	return nil
}

// ========== SYMLINKS ==========

func (clnt *ProcClnt) linkChildIntoParentDir(pid, procdir string) error {
	// Add symlink to child
	link := path.Join(proc.PARENTDIR, proc.PROCDIR)
	// May return file not found if parent exited.
	if err := clnt.Symlink([]byte(proc.GetProcDir()), link, 0777); err != nil && err.Error() != "file not found" {
		log.Printf("%v: Spawn Symlink child %v err %v\n", db.GetName(), link, err)
		return clnt.cleanupError(pid, procdir, err)
	}
	return nil
}

// ========== HELPERS ==========

// Attempt to cleanup procdir
func (clnt *ProcClnt) cleanupError(pid, procdir string, err error) error {
	clnt.removeChild(pid)
	clnt.removeProc(procdir)
	return err
}

func (clnt *ProcClnt) isKernelProc(pid string) bool {
	_, err := clnt.Stat(path.Join(proc.PROCDIR, proc.KERNEL_PROC))
	return err == nil
}

// ========== CHILDREN ==========

// Add a child to the current proc
func (clnt *ProcClnt) addChild(pid, procdir string) error {
	// Directory which holds link to child procdir
	childDir := path.Dir(proc.GetChildProcDir(pid))
	if err := clnt.Mkdir(childDir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir childs %v err %v\n", db.GetName(), childDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
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

	// Remove pid from my children now its status has been
	// collected we don't need to abandon it.
	if err := clnt.RmDir(childdir); err != nil {
		log.Printf("Error Remove %v in removeChild: %v", procdir, err)
		return fmt.Errorf("removeChild dir error %v", err)
	}
	return nil
}
