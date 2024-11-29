package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/sched/msched/proc/srv"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v kernelId dialproxy spproxydPID\nPassed: %v", os.Args[0], os.Args)
	}
	dialproxy, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("Can't parse dialproxy bool: %v", err)
	}
	scPID := sp.Tpid(os.Args[3])
	// ignore scheddIp
	if err := srv.RunProcSrv(os.Args[1], dialproxy, scPID); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
