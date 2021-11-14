package procclnt

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
)

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
	return MakeProcClnt(fsl, "pids", "")
}
