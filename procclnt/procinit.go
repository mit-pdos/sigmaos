package procclnt

import (
	"path"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rpc"
	sp "sigmaos/sigmap"
)

// Called by a sigmaOS process after being spawned
func NewProcClnt(fsl *fslib.FsLib) *ProcClnt {
	if fsl.ProcEnv().GetPrivileged() {
		db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
		fsl.NewRootMount(fsl.ProcEnv().ProcDir, proc.PROCDIR)
	}
	// If a schedd IP was specified for this proc, mount the RPC file directly.
	if fsl.ProcEnv().GetScheddAddr() != nil {
		addr := fsl.ProcEnv().GetScheddAddr()
		pn := path.Join(sp.SCHEDD, fsl.ProcEnv().GetKernelID(), rpc.RPC)
		db.DPrintf(db.PROCCLNT, "Mount[%v] %v as %v", addr, rpc.RPC, pn)
		start := time.Now()
		err := fsl.MountTree([]*sp.Taddr{addr}, rpc.RPC, pn)
		if err != nil {
			db.DFatalf("Err MountTree: %v", err)
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] MountTree latency: %v", fsl.ProcEnv().GetPID(), time.Since(start))
	}
	return newProcClnt(fsl, fsl.ProcEnv().GetPID(), fsl.ProcEnv().GetPrivileged())
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with NewProcClnt()
func NewProcClntInit(pid sp.Tpid, fsl *fslib.FsLib, program string) (*ProcClnt, error) {
	MountPids(fsl)
	// XXX needed?
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", sp.SCHEDDREL, sp.SCHEDDREL)
	if err := fsl.NewRootMount(sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		db.DPrintf(db.ALWAYS, "Error mounting schedd err %v\n", err)
		return nil, err
	}
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
	if err := fsl.NewRootMount(fsl.ProcEnv().ProcDir, proc.PROCDIR); err != nil {
		db.DPrintf(db.ALWAYS, "Error mounting procdir: %v", err)
		return nil, err
	}
	clnt := newProcClnt(fsl, pid, true)
	if err := clnt.MakeProcDir(pid, fsl.ProcEnv().ProcDir, false, proc.HSCHEDD); err != nil {
		db.DPrintf(db.ALWAYS, "NewProcClntInit: MakeProcDir err %v", err)
		// ignore error; the initial process may make several fslibs,
		// which each need to add mount point, but already has created
		// ProcDir.
		return clnt, nil
	}
	return clnt, nil
}

func MountPids(fsl *fslib.FsLib) error {
	fsl.NewRootMount(sp.KPIDS, sp.KPIDSREL)
	return nil
}
