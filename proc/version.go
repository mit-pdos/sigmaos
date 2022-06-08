package proc

import (
	"log"
	"runtime/debug"
)

var Version = "none"

func init() {
	if Version == "none" {
		debug.PrintStack()
		log.Fatalf("FATAL %v %v Version not set", GetName(), GetPid())
	}
}
