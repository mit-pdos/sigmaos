package procclnt

import (
	"log"
	"path"
	"strings"

	"runtime/debug"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
)

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	procdir := proc.GetProcDir()

	// XXX resolve mounts to find server?
	// Mount procdir
	tree := strings.TrimPrefix(procdir, "name/")
	if err := fsl.MountTree(fslib.Named(), tree, proc.PROCDIR); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), tree, proc.PROCDIR, err)
	}

	// Mount parentdir. May fail if parent already exited.
	parentdir := proc.GetParentDir()
	tree = strings.TrimPrefix(parentdir, "name/")
	if err := fsl.MountTree(fslib.Named(), tree, proc.PARENTDIR); err != nil {
		log.Printf("%v: Error mounting %v as %v err %v\n", db.GetName(), tree, proc.PARENTDIR, err)
	}

	if err := fsl.MountTree(fslib.Named(), "locks", "name/locks"); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting locks err %v\n", db.GetName(), err)
	}
	if err := fsl.MountTree(fslib.Named(), np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting procd err %v\n", db.GetName(), err)
	}
	return makeProcClnt(fsl, proc.GetPid())
}

// Called by tests to fake an initial process
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(fsl *fslib.FsLib, namedAddr []string) *ProcClnt {
	pid := proc.GenPid()
	proc.FakeProcEnv(pid, "NO_PROCD_IP", path.Join(proc.PIDS, pid), "")

	if err := fsl.MountTree(namedAddr, np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting procd err %v\n", db.GetName(), err)
	}

	MountPids(fsl, namedAddr)

	clnt := makeProcClnt(fsl, pid)
	clnt.MakeProcDir(pid, proc.GetProcDir(), false)

	tree := strings.TrimPrefix(proc.GetProcDir(), "name/")
	if err := fsl.MountTree(namedAddr, tree, proc.PROCDIR); err != nil {
		debug.PrintStack()
		s, _ := fsl.SprintfDir("pids")
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n%v", db.GetName(), tree, proc.PROCDIR, err, s)
	}

	return clnt
}

func MountPids(fsl *fslib.FsLib, namedAddr []string) error {
	// Make a pid directory for this initial proc
	if err := fsl.MountTree(namedAddr, proc.PIDS, proc.PIDS); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), proc.PIDS, proc.PIDS, err)
		return err
	}
	return nil
}
