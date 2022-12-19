package threadmgr

import (
	sp "sigmaos/sigmap"
)

type Op struct {
	Fm *sp.FcallMsg
	N  uint64 // Order in which this op was received.
}

func makeOp(fm *sp.FcallMsg, n uint64) *Op {
	return &Op{fm, n}
}
