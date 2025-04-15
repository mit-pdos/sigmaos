package procclnt

import (
	"path/filepath"
	"time"

	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

// Called by a sigmaOS process after being spawned
func NewProcClnt(fsl *fslib.FsLib) (*ProcClnt, error) {
	if fsl.ProcEnv().GetPrivileged() {
		db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
		fsl.NewRootMount(fsl.ProcEnv().ProcDir, proc.PROCDIR)
	}
	// If a schedd IP was specified for this proc, mount the RPC file directly.
	if ep, ok := fsl.ProcEnv().GetMSchedEndpoint(); ok {
		pn := filepath.Join(sp.MSCHED, fsl.ProcEnv().GetKernelID(), rpc.RPC)
		db.DPrintf(db.PROCCLNT, "Mount[%v] %v as %v", ep, rpc.RPC, pn)
		start := time.Now()
		err := fsl.MountTree(ep, rpc.RPC, pn)
		if err != nil {
			db.DPrintf(db.ERROR, "Err MountTree: ep %v err %v", ep, err)
			return nil, err
		}
		//db.DPrintf(db.TEST, "Mount schedd [%v] %v as %v time %v", ep, rpc.RPC, pn, time.Since(start))
		db.DPrintf(db.SPAWN_LAT, "[%v] MountTree latency: %v", fsl.ProcEnv().GetPID(), time.Since(start))
	}
	if ep, ok := fsl.ProcEnv().GetNamedEndpoint(); ok {
		start := time.Now()
		err := fsl.MountTree(ep, "", sp.NAMED)
		if err != nil {
			db.DPrintf(db.ERROR, "Err MountTree: ep %v err %v", ep, err)
			return nil, err
		}
		db.DPrintf(db.SPAWN_LAT, "Mount named [%v] time %v", ep, time.Since(start))
	}

	return newProcClnt(fsl, fsl.ProcEnv().GetPID(), fsl.ProcEnv().GetPrivileged(), fsl.ProcEnv().GetKernelID()), nil
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with NewProcClnt()
func NewProcClntInit(pid sp.Tpid, fsl *fslib.FsLib, program string) (*ProcClnt, error) {
	if err := MountPids(fsl); err != nil {
		db.DPrintf(db.ERROR, "error MountPids: %v", err)
		return nil, err
	}
	// XXX needed?
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", sp.MSCHEDREL, sp.MSCHEDREL)
	if err := fsl.NewRootMount(sp.MSCHEDREL, sp.MSCHEDREL); err != nil {
		db.DPrintf(db.ALWAYS, "Error mounting schedd err %v\n", err)
		return nil, err
	}
	db.DPrintf(db.PROCCLNT, "Mount %v as %v", fsl.ProcEnv().ProcDir, proc.PROCDIR)
	if err := fsl.NewRootMount(fsl.ProcEnv().ProcDir, proc.PROCDIR); err != nil {
		db.DPrintf(db.ALWAYS, "Error mounting procdir: %v", err)
		return nil, err
	}
	clnt := newProcClnt(fsl, pid, true, sp.NOT_SET)
	if err := clnt.MakeProcDir(pid, fsl.ProcEnv().ProcDir, false, proc.HMSCHED); err != nil {
		// If the error is not ErrExists, bail out.
		if !serr.IsErrCode(err, serr.TErrExists) {
			//	debug.PrintStack()
			db.DPrintf(db.ERROR, "Error MakeProcDir mkdir pid %v procdir %v err %v stack\n%v", pid, fsl.ProcEnv().ProcDir, err, string(debug.Stack()))
			return nil, err
		}
		db.DPrintf(db.PROCCLNT_ERR, "NewProcClntInit: MakeProcDir err %v", err)
		// ignore ErrExists: the initial test process may make several procclnts,
		// which each need to add mount point, but already has created
		// ProcDir.
		return clnt, nil
	}
	return clnt, nil
}

func MountPids(fsl *fslib.FsLib) error {
	return fsl.NewRootMount(sp.KPIDS, sp.KPIDSREL)
}
