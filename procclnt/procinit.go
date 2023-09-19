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
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
	// Mount procdir
	fsl.NewRootMount(fsl.ProcEnv().GetUname(), fsl.ProcEnv().ProcDir, proc.PROCDIR)

	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ParentDir, proc.PARENTDIR)
	// Mount parentdir. May fail if parent already exited.
	fsl.NewRootMount(fsl.ProcEnv().GetUname(), fsl.ProcEnv().ParentDir, proc.PARENTDIR)

	if err := fsl.NewRootMount(fsl.ProcEnv().GetUname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	return newProcClnt(fsl, fsl.ProcEnv().GetPID(), proc.PROCDIR)
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with NewProcClnt()
func NewProcClntInit(pid sp.Tpid, fsl *fslib.FsLib, program string) *ProcClnt {
	MountPids(fsl)

	db.DPrintf(db.PROCCLNT, "Mount %v as %v", sp.SCHEDDREL, sp.SCHEDDREL)
	if err := fsl.NewRootMount(fsl.ProcEnv().GetUname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := newProcClnt(fsl, pid, fsl.ProcEnv().ProcDir)
	clnt.NewProcDir(pid, fsl.ProcEnv().ProcDir, false, proc.HSCHEDD)

	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
	fsl.NewRootMount(fsl.ProcEnv().GetUname(), fsl.ProcEnv().ProcDir, proc.PROCDIR)
	return clnt
}

func MountPids(fsl *fslib.FsLib) error {
	fsl.NewRootMount(fsl.ProcEnv().GetUname(), sp.KPIDS, sp.KPIDS)
	return nil
}
