package main

import (
	"fmt"
	"os"

	"ulambda/procd"
)

//
// Requires Unix path to parent of "bin" directory (e.g., ulambda) so
// procd knows where to find its executables.  In the longer run this
// should probably be a lambda pathname, but since procd uses
// cmd.Run, which requires a Unix path, we use Unix pathnames.
//

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid\n", os.Args[0])
		os.Exit(1)
	}
	m := procd.MakeMonitor(os.Args[1])
	m.Work()
}
