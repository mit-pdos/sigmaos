package procclnt

import (
	"log"
	"os"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
)

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib) proc.ProcClnt {
	piddir := proc.GetPidDir()

	// XXX resolve mounts to find server?
	tree := strings.TrimPrefix(piddir, "name/")

	if err := fsl.MountTree(fslib.Named(), tree, "pids"); err != nil {
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), tree, "pids", err)
	}
	if err := fsl.MountTree(fslib.Named(), "runq", "name/runq"); err != nil {
		log.Fatalf("%v: Fatal error mounting runq err %v\n", db.GetName(), err)
	}
	if err := fsl.MountTree(fslib.Named(), "locks", "name/locks"); err != nil {
		log.Fatalf("%v: Fatal error mounting runq err %v\n", db.GetName(), err)
	}

	return MakeProcClntBase(fsl, piddir, proc.GetPid())
}

// Called by tests to fake an initial process
// XXX deduplicate with with Spawn()
func MakeProcClntInit(fsl *fslib.FsLib, NamedAddr []string) proc.ProcClnt {
	pid := proc.GenPid()
	os.Setenv("SIGMAPID", pid)
	if err := fsl.MountTree(NamedAddr, PIDS, PIDS); err != nil {
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), "pids", "pids", err)
	}
	d := PIDS + "/" + proc.GetPid()
	if err := fsl.Mkdir(d, 0777); err != nil {
		log.Fatalf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), d, err)
		return nil
	}

	d = PIDS + "/" + proc.GetPid() + "/" + CHILD
	if err := fsl.Mkdir(d, 0777); err != nil {
		log.Fatalf("%v: MakeProcClntInit childs %v err %v\n", db.GetName(), d, err)
		return nil
	}
	return MakeProcClntBase(fsl, "pids", "")
}
