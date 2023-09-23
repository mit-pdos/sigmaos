package procclnt

import (
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Called by a sigmaOS process after being spawned
func NewProcClnt(fsl *fslib.FsLib) *ProcClnt {
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ParentDir, proc.PARENTDIR)
	// Mount parentdir. May fail if parent already exited.
	fsl.NewRootMount(fsl.ProcEnv().GetUname(), fsl.ProcEnv().ParentDir, proc.PARENTDIR)
	if err := fsl.NewRootMount(fsl.ProcEnv().GetUname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}
	return newProcClnt(fsl, fsl.ProcEnv().GetPID(), fsl.ProcEnv().GetPrivileged())
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with NewProcClnt()
func NewProcClntInit(pid sp.Tpid, fsl *fslib.FsLib, program string) *ProcClnt {
	MountPids(fsl)
	// XXX needed?
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", sp.SCHEDDREL, sp.SCHEDDREL)
	if err := fsl.NewRootMount(fsl.ProcEnv().GetUname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting schedd err %v\n", err)
	}
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
	if err := fsl.NewRootMount(fsl.ProcEnv().GetUname(), fsl.ProcEnv().ProcDir, proc.PROCDIR); err != nil {
		db.DFatalf("Error mounting procdir: %v", err)
	}
	clnt := newProcClnt(fsl, pid, true)
	clnt.MakeProcDir(pid, fsl.ProcEnv().ProcDir, false, proc.HSCHEDD)
	return clnt
}

func MountPids(fsl *fslib.FsLib) error {
	fsl.NewRootMount(fsl.ProcEnv().GetUname(), sp.KPIDS, sp.KPIDSREL)
	return nil
}
