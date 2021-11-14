package procinit

import (
	"log"
	"os"
	"runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	// "ulambda/procbase"
	"ulambda/procclnt"
	"ulambda/procdep"
	"ulambda/procidem"
)

const (
	PROC_LAYERS = "PROC_LAYERS" // Environment variable in which to store layer configuration
)

const ( // Possible stackable layers. PROCBASE is always included by default
	PROCBASE = "PROCBASE"
	PROCIDEM = "PROCIDEM"
	PROCDEP  = "PROCDEP"
)

// Get proc layers from environment variables.
func GetProcLayersMap() map[string]bool {
	s := os.Getenv(PROC_LAYERS)
	// XXX Remove eventually, just here to make sure we don't miss anything
	if len(s) == 0 {
		debug.PrintStack()
		log.Fatalf("Error! Length 0 sched layers!")
	}
	ls := strings.Split(s, ",")
	layers := make(map[string]bool)
	for _, l := range ls {
		layers[l] = true
	}
	layers[PROCBASE] = true
	return layers
}

func GetProcLayersString() string {
	s := os.Getenv(PROC_LAYERS)
	// XXX Remove eventually, just here to make sure we don't miss anything
	if len(s) == 0 {
		debug.PrintStack()
		log.Fatalf("Error! Length 0 sched layers!")
	}
	return PROC_LAYERS + "=" + s
}

func SetProcLayers(layers map[string]bool) {
	os.Setenv(PROC_LAYERS, makeProcLayersString(layers))
}

func makeProcLayersString(layers map[string]bool) string {
	s := ""
	for l, _ := range layers {
		s += l
		s += ","
	}
	s = s[:len(s)-1]
	return s
}

// Make a generic ProcClnt with the desired layers.
func MakeProcClntBase(fsl *fslib.FsLib, layers map[string]bool, parent, pid string) proc.ProcClnt {
	var clnt proc.ProcClnt
	clnt = procclnt.MakeProcClnt(fsl, parent, pid)
	if _, ok := layers[PROCIDEM]; ok {
		clnt = procidem.MakeProcIdemClnt(fsl, clnt)
	}
	if _, ok := layers[PROCDEP]; ok {
		clnt = procdep.MakeProcDepClnt(fsl, clnt)
	}
	return clnt
}

// Called by a sigmaOS process after being spawned
func MakeProcClnt(fsl *fslib.FsLib, layers map[string]bool) proc.ProcClnt {
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

	return MakeProcClntBase(fsl, layers, piddir, proc.GetPid())
}

// Called by tests to fake an initial process
// XXX deduplicate with with spawn
func MakeProcClntInit(fsl *fslib.FsLib, layers map[string]bool, NamedAddr []string) proc.ProcClnt {
	pid := proc.GenPid()
	os.Setenv("SIGMAPID", pid)
	if err := fsl.MountTree(NamedAddr, procclnt.PIDS, procclnt.PIDS); err != nil {
		log.Fatalf("%v: Fatal error mounting %v as %v err %v\n", db.GetName(), "pids", "pids", err)
	}
	d := procclnt.PIDS + "/" + proc.GetPid()
	if err := fsl.Mkdir(d, 0777); err != nil {
		log.Fatalf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), d, err)
		return nil
	}

	d = procclnt.PIDS + "/" + proc.GetPid() + "/" + procclnt.CHILD
	if err := fsl.Mkdir(d, 0777); err != nil {
		log.Fatalf("%v: MakeProcClntInit childs %v err %v\n", db.GetName(), d, err)
		return nil
	}
	return MakeProcClntBase(fsl, layers, "pids", "")
}
