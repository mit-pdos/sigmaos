package threadmgr

import (
	np "ulambda/ninep"
)

type Op struct {
	fc      *np.Fcall
	replies chan *np.Fcall
}

func makeOp(fc *np.Fcall, replies chan *np.Fcall) *Op {
	return &Op{fc, replies}
}
