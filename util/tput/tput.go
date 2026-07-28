// Package tput formats sizes and throughputs for log output.
//
// These live in a leaf package (rather than in sigmaos/test, which re-exports
// them) so that procs can format a throughput without linking the test
// harness: sigmaos/test pulls in sigmaos/kernel -> dcontainer -> the Docker
// client, which costs every proc that links it ~1.5ms of package init and a
// lot of text.
package tput

import (
	"fmt"

	sp "sigmaos/sigmap"
)

// Mbyte returns sz in MB.
func Mbyte(sz sp.Tlength) float64 {
	return float64(sz) / float64(sp.MBYTE)
}

// Tput returns the throughput of sz bytes transferred in ms milliseconds, in
// MB/s.
func Tput(sz sp.Tlength, ms int64) float64 {
	t := float64(ms) / 1000
	return Mbyte(sz) / t
}

// TputStr formats the throughput of sz bytes transferred in ms milliseconds.
func TputStr(sz sp.Tlength, ms int64) string {
	return fmt.Sprintf("%.2fMB/s", Tput(sz, ms))
}
