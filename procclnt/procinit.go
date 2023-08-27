package procclnt

import (
	"path"
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	// Mount procdir
	fsl.MakeRootMount(fsl.Uname(), proc.GetProcDir(), proc.PROCDIR)

	// Mount parentdir. May fail if parent already exited.
	fsl.MakeRootMount(fsl.Uname(), proc.GetParentDir(), proc.PARENTDIR)

	if err := fsl.MakeRootMount(fsl.Uname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	return makeProcClnt(fsl, fsl.SigmaConfig().PID, proc.PROCDIR)
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(pid sp.Tpid, fsl *fslib.FsLib, program string) *ProcClnt {
	proc.FakeProcEnv(pid, program, path.Join(sp.KPIDSREL, pid.String()), "")
	MountPids(fsl, fsl.NamedAddr())

	if err := fsl.MakeRootMount(fsl.Uname(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := makeProcClnt(fsl, pid, proc.GetProcDir())
	clnt.MakeProcDir(pid, proc.GetProcDir(), false)

	fsl.MakeRootMount(fsl.Uname(), proc.GetProcDir(), proc.PROCDIR)
	return clnt
}

func MountPids(fsl *fslib.FsLib, namedAddr sp.Taddrs) error {
	fsl.MakeRootMount(fsl.Uname(), sp.KPIDSREL, sp.KPIDSREL)
	return nil
}
