package threadmgr

import (
	np "sigmaos/ninep"
)

type Op struct {
	Fc *np.Fcall
	N  uint64 // Order in which this op was received.
}

func makeOp(fc *np.Fcall, n uint64) *Op {
	return &Op{fc, n}
}
