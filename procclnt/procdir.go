package procclnt

import (
	"fmt"
	"log"
	"path"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/semclnt"
)

// For documentation on dir structure, see ulambda/proc/dir.go

func (clnt *ProcClnt) makeProcDir(pid, procdir string, isKernelProc bool) error {
	if err := clnt.Mkdir(procdir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), procdir, err)
		return err
	}
	childrenDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.Mkdir(childrenDir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir childrens %v err %v\n", db.GetName(), childrenDir, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}
	if isKernelProc {
		kprocFPath := path.Join(procdir, proc.KERNEL_PROC)
		if err := clnt.MakeFile(kprocFPath, 0777, np.OWRITE, []byte{}); err != nil {
			log.Printf("%v: MakeFile %v err %v", db.GetName(), kprocFPath, err)
			return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
	}
	return nil
}

func (clnt *ProcClnt) makeProcStatusSignals(pid, procdir string) error {
	err := clnt.MakePipe(path.Join(procdir, proc.RET_STATUS), 0777)
	if err != nil {
		log.Printf("%v: MakePipe %v err %v\n", db.GetName(), proc.RET_STATUS, err)
		return clnt.cleanupError(pid, procdir, fmt.Errorf("Spawn error %v", err))
	}

	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.START_SEM))
	semStart.Init()

	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	semEvict.Init()
	return nil
}
