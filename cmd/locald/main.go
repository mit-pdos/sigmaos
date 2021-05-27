package main

import (
	"fmt"
	"os"

	"ulambda/linuxsched"
	"ulambda/locald"
)

//
// Requires Unix path to parent of "bin" directory (e.g., ulambda) so
// locald knows where to find its executables.  In the longer run this
// should probably be a lambda pathname, but since locald uses
// cmd.Run, which requires a Unix path, we use Unix pathnames.
//

func main() {
	linuxsched.ScanTopology()
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v parent-of-bin <pprof-output-path>\n", os.Args[0])
		os.Exit(1)
	}
	pprofPath := ""
	if len(os.Args) >= 3 {
		pprofPath = os.Args[2]
	}
	utilPath := ""
	if len(os.Args) >= 4 {
		utilPath = os.Args[3]
	}
	ld := locald.MakeLocalD(os.Args[1], pprofPath, utilPath)
	ld.Work()
}
