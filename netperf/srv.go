package netperf

import (
	"fmt"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunSrv(args []string) error {
	if len(args) != 3 {
		db.DFatalf("Usage: netperf-srv OP_TYPE ADDR NTRIAL")
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return err
	}
	op := args[0]
	addr, err := sp.NewTaddrFromString(args[1], sp.INNER_CONTAINER_IP)
	if err != nil {
		db.DFatalf("Err NewAddr: %v", err)
	}
	ntrial, err := strconv.Atoi(args[2])
	if err != nil {
		db.DFatalf("Invalid ntrial: %v", err)
	}
	ch := make(chan bool)
	switch op {
	case "dial":
		go func() {
			if err := SrvDialNetProxy(ch, ntrial, sc.GetNetProxyClnt(), addr, sp.INTERNAL_EP); err != nil {
				db.DFatalf("Err SrvDialNetProxy: %v", err)
			}
			ch <- true
		}()
	default:
		db.DFatalf("unexpected op %v", op)
	}
	// Wait for server to start listening before marking self as started
	<-ch
	if err := sc.Started(); err != nil {
		return fmt.Errorf("RunSrv couldn't start %v", args)
	}
	// Wait for the server to finish
	<-ch
	sc.ClntExitOK()
	return nil
}
