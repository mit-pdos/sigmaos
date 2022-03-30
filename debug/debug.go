package debug

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"ulambda/proc"
)

const ALWAYS = "STATUS"

func init() {
	// XXX may want to set log.Ldate when not debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds)
}

//
// Debug output is controled by SIGMADEBUG environment variable, which
// can be a list of labels (e.g., "RPC;PATHCLNT").
//

func debugLabels() map[string]bool {
	m := make(map[string]bool)
	s := os.Getenv("SIGMADEBUG")
	if s == "" {
		return m
	}
	labels := strings.Split(s, ";")
	for _, l := range labels {
		m[l] = true
	}
	return m
}

func DPrintf(label string, format string, v ...interface{}) {
	m := debugLabels()
	if _, ok := m[label]; ok || label == ALWAYS {
		log.Printf("%v %v %v", proc.GetName(), label, fmt.Sprintf(format, v...))
	}
}

func DFatalf(format string, v ...interface{}) {
	// Get info for the caller.
	pc, file, line, ok := runtime.Caller(1)
	fnDetails := runtime.FuncForPC(pc)
	if ok && fnDetails != nil {
		log.Fatalf("FATAL %v %v %v:%v %v", proc.GetName(), fnDetails.Name(), file, line, fmt.Sprintf(format, v...))
	} else {
		log.Fatalf("FATAL %v (missing details) %v", proc.GetName(), fmt.Sprintf(format, v...))
	}
}
