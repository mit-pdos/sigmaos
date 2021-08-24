package main

import (
	"fmt"
	"os"

	"ulambda/linuxsched"
	"ulambda/procd"
)

//
// Requires Unix path to parent of "bin" directory (e.g., ulambda) so
// procd knows where to find its executables.  In the longer run this
// should probably be a lambda pathname, but since procd uses
// cmd.Run, which requires a Unix path, we use Unix pathnames.
//

func main() {
	linuxsched.ScanTopology()
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v parent-of-bin pid <pprof-output-path> <util-path>\n", os.Args[0])
		os.Exit(1)
	}
	pprofPath := ""
	if len(os.Args) >= 4 {
		pprofPath = os.Args[3]
	}
	utilPath := ""
	if len(os.Args) >= 5 {
		utilPath = os.Args[4]
	}
	pd := procd.MakeProcd(os.Args[1], os.Args[2], pprofPath, utilPath)
	pd.Work()
}
