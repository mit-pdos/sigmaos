package procinit

import (
	"log"
	"os"
	"runtime/debug"
	"strings"

	"ulambda/fslib"
	"ulambda/proc"
	// "ulambda/procbase"
	"ulambda/procbasev1"
	"ulambda/procdep"
	"ulambda/procidem"
)

// Can return "" for test programs that make a procclnt
func GetPid() string {
	return os.Getenv("SIGMAPID")
}

// Can return "" for test programs that make a procclnt
func GetPidDir() string {
	return os.Getenv("SIGMAPIDDIR")
}

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
	clnt = procbasev1.MakeProcBaseClnt(fsl, parent, pid)
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
	return MakeProcClntBase(fsl, layers, GetPidDir(), GetPid())
}

// Called by tests to fake an initial process
func MakeProcClntInit(fsl *fslib.FsLib, layers map[string]bool) proc.ProcClnt {
	return MakeProcClntBase(fsl, layers, "pids", "")
}
