package procclnt

import (
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
	// Mount procdir
	fsl.MakeRootMount(fsl.Uname(), fsl.ProcEnv().ProcDir, proc.PROCDIR)

	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ParentDir, proc.PARENTDIR)
	// Mount parentdir. May fail if parent already exited.
	fsl.MakeRootMount(fsl.Uname(), fsl.ProcEnv().ParentDir, proc.PARENTDIR)

	if err := fsl.MakeRootMount(fsl.Uname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	return makeProcClnt(fsl, fsl.ProcEnv().GetPID(), proc.PROCDIR)
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(pid sp.Tpid, fsl *fslib.FsLib, program string) *ProcClnt {
	MountPids(fsl, fsl.NamedAddr())

	if err := fsl.MakeRootMount(fsl.Uname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := makeProcClnt(fsl, pid, fsl.ProcEnv().ProcDir)
	clnt.MakeProcDir(pid, fsl.ProcEnv().ProcDir, false)

	fsl.MakeRootMount(fsl.Uname(), fsl.ProcEnv().ProcDir, proc.PROCDIR)
	return clnt
}

func MountPids(fsl *fslib.FsLib, namedAddr sp.Taddrs) error {
	fsl.MakeRootMount(fsl.Uname(), sp.KPIDSREL, sp.KPIDSREL)
	return nil
}
