package procclnt

import (
	"log"
	"path"
	"strings"

	"runtime/debug"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
)

// Right now mounts don't resolve to find the server. So, get the server addr
// from the path for now.
func splitMountServerAddrPath(fsl *fslib.FsLib, dpath string) ([]string, string) {
	p := strings.Split(dpath, "/")
	for i := len(p) - 1; i >= 0; i-- {
		if strings.Contains(p[i], ":") {
			return []string{p[i]}, path.Join(p[i+1:]...)
		}
	}
	return fslib.Named(), dpath
}

func mountDir(fsl *fslib.FsLib, dpath string, mountPoint string) {
	tree := strings.TrimPrefix(dpath, "name/")
	addr, splitPath := splitMountServerAddrPath(fsl, tree)
	if err := fsl.MountTree(addr, splitPath, mountPoint); err != nil {
		if mountPoint == proc.PARENTDIR {
			log.Printf("%v: Error mounting %v/%v as %v err %v\n", proc.GetProgram(), addr, splitPath, mountPoint, err)
		} else {
			debug.PrintStack()
			log.Fatalf("%v: FATAL error mounting %v/%v as %v err %v\n", proc.GetProgram(), addr, splitPath, mountPoint, err)
		}
	}
}

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	// XXX resolve mounts to find server?
	// Mount procdir
	mountDir(fsl, proc.GetProcDir(), proc.PROCDIR)

	// Mount parentdir. May fail if parent already exited.
	mountDir(fsl, proc.GetParentDir(), proc.PARENTDIR)

	if err := fsl.MountTree(fslib.Named(), np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: FATAL error mounting procd err %v\n", proc.GetProgram(), err)
	}
	return makeProcClnt(fsl, proc.GetPid())
}

// Called by tests to fake an initial process
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(fsl *fslib.FsLib, uname string, namedAddr []string) *ProcClnt {
	pid := proc.GenPid()
	proc.FakeProcEnv(pid, uname, "", path.Join(proc.PIDS, pid), "")

	if err := fsl.MountTree(namedAddr, np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: FATAL error mounting procd err %v\n", proc.GetProgram(), err)
	}

	MountPids(fsl, namedAddr)

	clnt := makeProcClnt(fsl, pid)
	clnt.MakeProcDir(pid, proc.GetProcDir(), false)

	tree := strings.TrimPrefix(proc.GetProcDir(), "name/")
	if err := fsl.MountTree(namedAddr, tree, proc.PROCDIR); err != nil {
		debug.PrintStack()
		s, _ := fsl.SprintfDir("pids")
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n%v", proc.GetProgram(), tree, proc.PROCDIR, err, s)
	}

	return clnt
}

func MountPids(fsl *fslib.FsLib, namedAddr []string) error {
	// Make a pid directory for this initial proc
	if err := fsl.MountTree(namedAddr, proc.PIDS, proc.PIDS); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: FATAL error mounting %v as %v err %v\n", proc.GetProgram(), proc.PIDS, proc.PIDS, err)
		return err
	}
	if err := fsl.MountTree(namedAddr, proc.KPIDS, proc.KPIDS); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: FATAL error mounting %v as %v err %v\n", proc.GetProgram(), proc.KPIDS, proc.KPIDS, err)
		return err
	}
	return nil
}
