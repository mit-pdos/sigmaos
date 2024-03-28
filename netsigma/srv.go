package netsigma

import (
	"net"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func Listen(pe *proc.ProcEnv, addr *sp.Taddr) (net.Listener, error) {
	// TODO: if ProcEnv has usesigmaclntd switched on, then get listener from
	// sigmaclntd
	l, err := net.Listen("tcp", addr.IPPort())
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Listen on addr %v: err %v", addr, err)
	}
	db.DPrintf(db.NETSIGMA, "Listen on addr %v res addr %v", addr, l.Addr())
	return l, err
}
