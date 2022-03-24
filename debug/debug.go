package debug

import (
	"fmt"
	"log"
	"os"
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

// Deprecated; use DLPrintf
func DPrintf(format string, v ...interface{}) {
	m := debugLabels()
	if len(m) != 0 {
		log.Printf("%v: %v", proc.GetName(), fmt.Sprintf(format, v...))
	}
}

func DLPrintf(label string, format string, v ...interface{}) {
	m := debugLabels()
	if _, ok := m[label]; ok || label == ALWAYS {
		log.Printf("%v %v %v", proc.GetName(), label, fmt.Sprintf(format, v...))
	}
}
