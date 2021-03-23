package main

import (
	"fmt"
	"os"

	"ulambda/locald"
)

//
// Requires Unix path to parent of "bin" directory (e.g., ulambda) so
// locald knows where to find its executables.  In the longer run this
// should probably be a lambda pathname, but since locald uses
// cmd.Run, which requires a Unix path, we use Unix pathnames.
//

func main() {
	locald.ScanTopology()
	//	ti, err := locald.ScanTopology()
	//	if err == nil {
	//		locald.PrintTopology(ti)
	//	}
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v parent-of-bin\n", os.Args[0])
		os.Exit(1)
	}
	ld := locald.MakeLocalD(os.Args[1])
	ld.Work()
}
