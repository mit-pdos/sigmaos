package netsigma

import (
	"net"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func Dial(pe *proc.ProcEnv, addr *sp.Taddr) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", addr.IPPort(), sp.Conf.Session.TIMEOUT/10)
	// TODO: if ProcEnv has usesigmaclntd switched on, then get conn from
	// sigmaclntd
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Dial addr %v: err %v", addr, err)
	}
	db.DPrintf(db.NETSIGMA, "Dial addr %v", addr)
	return c, err
}
