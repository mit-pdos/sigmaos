package debug

import (
	"fmt"
	"log"
	"os"

	"ulambda/proc"
)

// XXX maybe a list of levels?
func debug() string {
	return os.Getenv("SIGMADEBUG")
}

func DPrintf(format string, v ...interface{}) {
	if debug() != "" {
		log.Printf("%v: %v", proc.GetName(), fmt.Sprintf(format, v...))
	}
}

func DLPrintf(label string, format string, v ...interface{}) {
	if debug() == label {
		log.Printf("%v %v %v", proc.GetName(), label, fmt.Sprintf(format, v...))
	}
}
