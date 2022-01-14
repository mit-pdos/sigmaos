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
	tree := strings.TrimPrefix(procdir, "name/")

	if err := fsl.MountTree(fslib.Named(), tree, proc.PIDS); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), tree, proc.PIDS, err)
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
func MakeProcClntInit(fsl *fslib.FsLib, NamedAddr []string) *ProcClnt {
	proc.SetPid(proc.GenPid())

	if err := fsl.MountTree(NamedAddr, np.PROCDREL, np.PROCDREL); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting procd err %v\n", db.GetName(), err)
	}

	// Make a pid directory for this initial proc
	if err := fsl.MountTree(NamedAddr, proc.PIDS, proc.PIDS); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), proc.PIDS, proc.PIDS, err)
	}
	d := path.Join(proc.PIDS, proc.GetPid())
	if err := fsl.Mkdir(d, 0777); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), d, err)
		return nil
	}
	d = path.Join(proc.PIDS, proc.GetPid(), proc.CHILDREN)
	if err := fsl.Mkdir(d, 0777); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: MakeProcClntInit childs %v err %v\n", db.GetName(), d, err)
		return nil
	}

	return makeProcClnt(fsl, proc.GetPid())
}
