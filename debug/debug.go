package debug

import (
	"fmt"
	"log"
	"os"

	"ulambda/proc"
)

func isDebug() bool {
	uldebug := os.Getenv("SIGMADEBUG")
	return uldebug != ""
}

func DPrintf(format string, v ...interface{}) {
	if isDebug() {
		log.Printf("%v: %v", proc.GetProgram(), fmt.Sprintf(format, v...))
	}
}

func DLPrintf(label string, format string, v ...interface{}) {
	if isDebug() {
		log.Printf("%v %v %v", proc.GetProgram(), label, fmt.Sprintf(format, v...))
	}
}
