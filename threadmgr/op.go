package threadmgr

import (
	np "sigmaos/sigmap"
)

type Op struct {
	Fm *np.FcallMsg
	N  uint64 // Order in which this op was received.
}

func makeOp(fm *np.FcallMsg, n uint64) *Op {
	return &Op{fm, n}
}
