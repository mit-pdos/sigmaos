// The debug package allow callers to control debug output through the
// SIGMADEBUG environment variable, which can be a list of labels
// (e.g., "RPC;PATHCLNT").  If the goal is to log debugging output for
// a few programs, set SIGMADEBUGPROCS to the names of the programs
// (e.g., "realmd;named")
package debug

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"sigmaos/proc"
)

var labels map[Tselector]bool
var doLog bool

func init() {
	s := time.Now()
	// XXX may want to set log.Ldate when not debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	labelstr := proc.GetLabelsEnv(proc.SIGMADEBUG)
	procstr := proc.GetLabelsEnv(proc.SIGMADEBUGPROCS)
	labels = make(map[Tselector]bool, len(labelstr))
	for k, v := range labelstr {
		labels[Tselector(k)] = v
	}
	if len(procstr) == 0 {
		doLog = true
	} else {
		for p, _ := range procstr {
			doLog = isLoggingProc(proc.GetSigmaDebugPid(), p)
			if doLog {
				log.Printf("Logging %v", proc.GetSigmaDebugPid())
				break
			}
		}
	}
	DPrintf(SPAWN_LAT, "[%v] debug init pkg: %v", proc.GetSigmaDebugPid(), time.Since(s))
}

// Sometimes, converting pointers to call DPrintf is very expensive (and occurs
// often, e.g., in the session layer). So, the function below can be called to
// efficiently check if the DPrintf would succeed.
func WillBePrinted(label Tselector) bool {
	_, ok := labels[label]
	return ok || label == ALWAYS
}

func isLoggingProc(pid, proc string) bool {
	return strings.HasPrefix(pid, proc)
}

func DPrintf(label Tselector, format string, v ...interface{}) {
	pid := proc.GetSigmaDebugPid()
	if _, ok := labels[label]; (ok && doLog) || label == ALWAYS {
		log.Printf("%v %v %v", pid, label, fmt.Sprintf(format, v...))
	} else {
		if label == ERROR {
			log.Printf("%v %v %v\nStack trace:\n%v", pid, label, fmt.Sprintf(format, v...), string(debug.Stack()))
		}
	}
}

func DFatalf(format string, v ...interface{}) {
	// Get info for the caller.
	pc, _, _, ok := runtime.Caller(1)
	fnDetails := runtime.FuncForPC(pc)
	fnName := strings.TrimPrefix(fnDetails.Name(), "sigmaos/")
	debug.PrintStack()
	if ok && fnDetails != nil {
		log.Fatalf("FATAL %v %v Err: %v", proc.GetSigmaDebugPid(), fnName, fmt.Sprintf(format, v...))
	} else {
		log.Fatalf("FATAL %v (missing details) %v", proc.GetSigmaDebugPid(), fmt.Sprintf(format, v...))
	}
}

func LsDir(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		DFatalf("readdir %v", err)
	}
	s := fmt.Sprintf("lsdir %q: [", path)
	for _, e := range entries {
		s += fmt.Sprintf("%q,", e.Name())
	}
	s += "]"
	return s
}
