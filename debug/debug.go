package debug

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"sigmaos/proc"
)

//
// Debug output is controled by SIGMADEBUG environment variable, which
// can be a list of labels (e.g., "RPC;PATHCLNT").
//

var labels map[Tselector]bool

func init() {
	// XXX may want to set log.Ldate when not debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	labelstr := proc.GetLabels(proc.SIGMADEBUG)
	labels = make(map[Tselector]bool, len(labelstr))
	for k, v := range labelstr {
		labels[Tselector(k)] = v
	}
}

// Sometimes, converting pointers to call DPrintf is very expensive (and occurs
// often, e.g., in the session layer). So, the function below can be called to
// efficiently check if the DPrintf would succeed.
func WillBePrinted(label Tselector) bool {
	_, ok := labels[label]
	return ok || label == ALWAYS
}

func DPrintf(label Tselector, format string, v ...interface{}) {
	if _, ok := labels[label]; ok || label == ALWAYS {
		log.Printf("%v %v %v", proc.GetName(), label, fmt.Sprintf(format, v...))
	}
}

func DFatalf(format string, v ...interface{}) {
	// Get info for the caller.
	pc, _, _, ok := runtime.Caller(1)
	fnDetails := runtime.FuncForPC(pc)
	fnName := strings.TrimLeft(fnDetails.Name(), "sigmaos/")
	if ok && fnDetails != nil {
		log.Fatalf("FATAL %v %v Err: %v", proc.GetName(), fnName, fmt.Sprintf(format, v...))
	} else {
		log.Fatalf("FATAL %v (missing details) %v", proc.GetName(), fmt.Sprintf(format, v...))
	}
}
