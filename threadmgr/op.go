package threadmgr

import (
	"sigmaos/sessp"
)

type Op struct {
	Fm *sessp.FcallMsg
	N  uint64 // Order in which this op was received.
}

func makeOp(fm *sessp.FcallMsg, n uint64) *Op {
	return &Op{fm, n}
}
