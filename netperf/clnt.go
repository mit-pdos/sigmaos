package netperf

import (
	"fmt"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunClnt(args []string) error {
	if len(args) != 3 {
		db.DFatalf("Usage: netperf-clnt OP_TYPE ADDR NTRIAL")
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
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})
	ntrial, err := strconv.Atoi(args[2])
	if err != nil {
		db.DFatalf("Invalid ntrial: %v", err)
	}
	if err := sc.Started(); err != nil {
		return fmt.Errorf("RunSrv couldn't start %v", args)
	}
	var perfStr string
	switch op {
	case "dial":
		if perfStr, err = ClntDialNetProxy(ntrial, sc.GetDialProxyClnt(), ep); err != nil {
			db.DFatalf("Err SrvDialNetProxy: %v", err)
		}
	default:
		db.DFatalf("unexpected op %v", op)
	}
	sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, perfStr, nil))
	return nil
}
