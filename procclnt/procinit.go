package procclnt

import (
	"path"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Right now mounts don't resolve to find the server. So, get the server addr
// from the path for now.
func splitMountServerAddrPath(fsl *fslib.FsLib, namedAddrs sp.Taddrs, dpath string) (sp.Taddrs, string) {
	p := strings.Split(dpath, "/")
	for i := len(p) - 1; i >= 0; i-- {
		if strings.Contains(p[i], ":") {
			return sp.MkTaddrs([]string{p[i]}), path.Join(p[i+1:]...)
		}
	}
	return namedAddrs, dpath
}

func mountDir(fsl *fslib.FsLib, namedAddrs sp.Taddrs, dpath string, mountPoint string) {
	tree := strings.TrimPrefix(dpath, "name/")
	addr, splitPath := splitMountServerAddrPath(fsl, namedAddrs, tree)
	if err := fsl.MountTree(addr, splitPath, mountPoint); err != nil {
		if mountPoint == proc.PARENTDIR {
			db.DPrintf(db.PROCCLNT_ERR, "Error mounting %v/%v as %v err %v\n", addr, splitPath, mountPoint, err)
		} else {
			debug.PrintStack()
			sts, err2 := fsl.GetDir(sp.SCHEDD)
			db.DFatalf("Error mounting %v/%v as %v err %v\nsts:%v err2:%v", addr, splitPath, mountPoint, err, sp.Names(sts), err2)
		}
	}
}

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	// XXX resolve mounts to find server?
	// Mount procdir
	mountDir(fsl, fsl.NamedAddr(), proc.GetProcDir(), proc.PROCDIR)

	// Mount parentdir. May fail if parent already exited.
	mountDir(fsl, fsl.NamedAddr(), proc.GetParentDir(), proc.PARENTDIR)

	if err := fsl.MountTree(fsl.NamedAddr(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}
	return makeProcClnt(fsl, proc.GetPid(), proc.PROCDIR)
}

// Fake an initial process for, for example, tests.
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(pid proc.Tpid, fsl *fslib.FsLib, program string) *ProcClnt {
	proc.FakeProcEnv(pid, program, path.Join(sp.KPIDSREL, pid.String()), "")
	MountPids(fsl, fsl.NamedAddr())

	if err := fsl.MountTree(fsl.NamedAddr(), sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := makeProcClnt(fsl, pid, proc.GetProcDir())
	clnt.MakeProcDir(pid, proc.GetProcDir(), false)

	mountDir(fsl, fsl.NamedAddr(), proc.GetProcDir(), proc.PROCDIR)

	return clnt
}

func MountPids(fsl *fslib.FsLib, namedAddr sp.Taddrs) error {
	mountDir(fsl, namedAddr, sp.KPIDSREL, sp.KPIDSREL)
	return nil
}

// XXX REMOVE THIS AFTER DEADLINE PUSH
func MakeProcClntTmp(fsl *fslib.FsLib, namedAddr sp.Taddrs) *ProcClnt {
	MountPids(fsl, namedAddr)
	if err := fsl.MountTree(namedAddr, sp.SCHEDDREL, sp.SCHEDDREL); err != nil {
		debug.PrintStack()
		db.DFatalf("error mounting procd err %v\n", err)
	}

	clnt := makeProcClnt(fsl, proc.GetPid(), proc.GetProcDir())

	mountDir(fsl, namedAddr, proc.GetProcDir(), proc.PROCDIR)

	return clnt
}
