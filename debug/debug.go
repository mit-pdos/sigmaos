package debug

import (
	"fmt"
	"log"
	"os"
	"strings"

	"ulambda/proc"
)

// XXX maybe a list of levels?
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

func DPrintf(format string, v ...interface{}) {
	m := debugLabels()
	if len(m) != 0 {
		log.Printf("%v: %v", proc.GetName(), fmt.Sprintf(format, v...))
	}
}

func DLPrintf(label string, format string, v ...interface{}) {
	m := debugLabels()
	if _, ok := m[label]; ok {
		log.Printf("%v %v %v", proc.GetName(), label, fmt.Sprintf(format, v...))
	}
}
