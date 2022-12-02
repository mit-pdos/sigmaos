package procclnt

import (
	"path"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/sigmap"
	"sigmaos/proc"
)

// Right now mounts don't resolve to find the server. So, get the server addr
// from the path for now.
func splitMountServerAddrPath(fsl *fslib.FsLib, namedAddrs []string, dpath string) ([]string, string) {
	p := strings.Split(dpath, "/")
	for i := len(p) - 1; i >= 0; i-- {
		if strings.Contains(p[i], ":") {
			return []string{p[i]}, path.Join(p[i+1:]...)
		}
	}
	return namedAddrs, dpath
}

func mountDir(fsl *fslib.FsLib, namedAddrs []string, dpath string, mountPoint string) {
	tree := strings.TrimPrefix(dpath, "name/")
	addr, splitPath := splitMountServerAddrPath(fsl, namedAddrs, tree)
	if err := fsl.MountTree(addr, splitPath, mountPoint); err != nil {
		if mountPoint == proc.PARENTDIR {
			db.DPrintf("PROCCLNT_ERR", "Error mounting %v/%v as %v err %v\n", addr, splitPath, mountPoint, err)
		} else {
			debug.PrintStack()
			db.DFatalf("error mounting %v/%v as %v err %v", addr, splitPath, mountPoint, err)
		}
	}
}

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	// XXX resolve mounts to find server?
	// Mount procdir
	mountDir(fsl, fslib.Named(), proc.GetProcDir(), proc.PROCDIR)

	// Mount parentdir. May fail if parent already exited.
	mountDir(fsl, fslib.Named(), proc.GetParentDir(), proc.PARENTDIR)

	if err := fsl.MountTree(fslib.Named(), np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}
	return makeProcClnt(fsl, proc.GetPid(), proc.PROCDIR)
}

// Called by tests to fake an initial process
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(pid proc.Tpid, fsl *fslib.FsLib, uname string, namedAddr []string) *ProcClnt {
	proc.FakeProcEnv(pid, uname, "", path.Join(proc.KPIDS, pid.String()), "")
	MountPids(fsl, namedAddr)

	if err := fsl.MountTree(namedAddr, np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := makeProcClnt(fsl, pid, proc.GetProcDir())
	clnt.MakeProcDir(pid, proc.GetProcDir(), false)

	mountDir(fsl, namedAddr, proc.GetProcDir(), proc.PROCDIR)

	return clnt
}

func MountPids(fsl *fslib.FsLib, namedAddr []string) error {
	// Make a pid directory for this initial proc
	if err := fsl.MountTree(namedAddr, proc.KPIDS, proc.KPIDS); err != nil {
		db.DFatalf("error mounting %v as %v err %v\n", proc.KPIDS, proc.KPIDS, err)
		return err
	}
	return nil
}

// XXX REMOVE THIS AFTER DEADLINE PUSH
func MakeProcClntTmp(fsl *fslib.FsLib, namedAddr []string) *ProcClnt {
	MountPids(fsl, namedAddr)
	if err := fsl.MountTree(namedAddr, np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := makeProcClnt(fsl, proc.GetPid(), proc.GetProcDir())

	mountDir(fsl, namedAddr, proc.GetProcDir(), proc.PROCDIR)

	return clnt
}
