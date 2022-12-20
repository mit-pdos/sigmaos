package threadmgr

import (
	"sigmaos/fcall"
)

type Op struct {
	Fm *fcall.FcallMsg
	N  uint64 // Order in which this op was received.
}

func makeOp(fm *fcall.FcallMsg, n uint64) *Op {
	return &Op{fm, n}
}
