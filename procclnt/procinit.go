package procclnt

import (
	"log"
	"strings"

	"runtime/debug"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
)

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) *ProcClnt {
	piddir := proc.GetPidDir()

	// XXX resolve mounts to find server?
	tree := strings.TrimPrefix(piddir, "name/")

	if err := fsl.MountTree(fslib.Named(), tree, "pids"); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), tree, "pids", err)
	}
	if err := fsl.MountTree(fslib.Named(), "locks", "name/locks"); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting locks err %v\n", db.GetName(), err)
	}
	if err := fsl.MountTree(fslib.Named(), named.PROCDDIR, named.PROCDDIR); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting procd err %v\n", db.GetName(), err)
	}
	return makeProcClnt(fsl, piddir, proc.GetPid())
}

// Called by tests to fake an initial process
// XXX deduplicate with Spawn()
// XXX deduplicate with MakeProcClnt()
func MakeProcClntInit(fsl *fslib.FsLib, NamedAddr []string) *ProcClnt {
	proc.SetPid(proc.GenPid())

	if err := fsl.MountTree(NamedAddr, named.PROCDDIR, named.PROCDDIR); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting procd err %v\n", db.GetName(), err)
	}

	// Make a pid directory for this initial proc
	if err := fsl.MountTree(NamedAddr, PIDS, PIDS); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), "pids", "pids", err)
	}
	d := PIDS + "/" + proc.GetPid()
	if err := fsl.Mkdir(d, 0777); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), d, err)
		return nil
	}
	d = PIDS + "/" + proc.GetPid() + "/" + CHILD
	if err := fsl.Mkdir(d, 0777); err != nil {
		debug.PrintStack()
		log.Fatalf("%v: MakeProcClntInit childs %v err %v\n", db.GetName(), d, err)
		return nil
	}

	return makeProcClnt(fsl, "pids", proc.GetPid())
}
