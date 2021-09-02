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
	PROC_LAYERS = "PROC_LAYERS" // Environment variable in which to store layer configuration
)

const ( // Possible stackable layers. BASEPROC is always included by default
	BASEPROC = "BASEPROC"
	IDEMPROC = "IDEMPROC"
	DEPPROC  = "DEPPROC"
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
	layers[BASEPROC] = true
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
func MakeProcClnt(fsl *fslib.FsLib, layers map[string]bool) proc.ProcClnt {
	var ctl proc.ProcClnt
	ctl = baseproc.MakeBaseProcClnt(fsl)
	if _, ok := layers[IDEMPROC]; ok {
		ctl = idemproc.MakeIdemProcClnt(fsl, ctl)
	}
	if _, ok := layers[DEPPROC]; ok {
		ctl = depproc.MakeDepProcClnt(fsl, ctl)
	}
	return ctl
}
