package procinit

import (
	"log"
	"os"
	"runtime/debug"
	"strings"

	"ulambda/baseproc"
	"ulambda/depproc"
	"ulambda/fslib"
	"ulambda/idemproc"
	"ulambda/proc"
)

const (
	SCHED_LAYERS = "SCHED_LAYERS" // Environment variable in which to store layer configuration
)

// XXX Rename
const ( // Possible stackable layers. BASE_SCHED is always included by default
	BASESCHED = "BASESCHED"
	IDEMSCHED = "IDEMSCHED"
	DEPSCHED  = "DEPSCHED"
)

// Get proc layers from environment variables.
func GetProcLayers() map[string]bool {
	s := os.Getenv(SCHED_LAYERS)
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
	layers[BASESCHED] = true
	return layers
}

func CopyProcLayers() string {
	s := os.Getenv(SCHED_LAYERS)
	// XXX Remove eventually, just here to make sure we don't miss anything
	if len(s) == 0 {
		debug.PrintStack()
		log.Fatalf("Error! Length 0 sched layers!")
	}
	return s
}

// XXX Rename
func MakeProcLayers(layers map[string]bool) string {
	s := SCHED_LAYERS + "="
	for l, _ := range layers {
		s += l
		s += ","
	}
	s = s[:len(s)-1]
	return s
}

// Make a generic ProcCtl with the desired layers.
func MakeProcCtl(fsl *fslib.FsLib, layers map[string]bool) proc.ProcCtl {
	var ctl proc.ProcCtl
	ctl = baseproc.MakeBaseProcCtl(fsl)
	if _, ok := layers[IDEMSCHED]; ok {
		ctl = idemproc.MakeIdemProcCtl(fsl, ctl)
	}
	if _, ok := layers[DEPSCHED]; ok {
		ctl = depproc.MakeDepProcCtl(fsl, ctl)
	}
	return ctl
}
