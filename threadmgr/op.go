package threadmgr

import (
	np "ulambda/ninep"
)

type Op struct {
	fc      *np.Fcall
	replies chan *np.Fcall
	n       uint64 // Order in which this op was received.
}

func makeOp(fc *np.Fcall, replies chan *np.Fcall, n uint64) *Op {
	return &Op{fc, replies, n}
}
