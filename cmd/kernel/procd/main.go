package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/sched/msched/proc/srv"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 7 {
		db.DFatalf("Usage: %v kernelId dialproxy gvisor spproxydPID wasmdPID enableBlink\nPassed: %v", os.Args[0], os.Args)
	}
	dialproxy, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("Can't parse dialproxy bool: %v", err)
	}
	gvisor, err := strconv.ParseBool(os.Args[3])
	if err != nil {
		db.DFatalf("Can't parse gvisor bool: %v", err)
	}
	scPID := sp.Tpid(os.Args[4])
	wasmdPID := sp.Tpid(os.Args[5])
	enableBlink, err := strconv.ParseBool(os.Args[6])
	if err != nil {
		db.DFatalf("Can't parse enableBlink bool: %v", err)
	}
	if err := srv.RunProcSrv(os.Args[1], dialproxy, gvisor, scPID, wasmdPID, enableBlink); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
