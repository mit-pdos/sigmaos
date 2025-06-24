package proc

import (
	sessp "sigmaos/session/proto"
)

func newInitializationRPC(pn string, iov sessp.IoVec, nOutIOV uint64) *InitializationRPC {
	return &InitializationRPC{
		TargetPathname:  pn,
		MarshaledRPCIOV: iov.ToByteSlices(),
		NOutIOV:         nOutIOV,
	}
}

func (rpc *InitializationRPC) GetInputIOV() sessp.IoVec {
	return sessp.IoVec(rpc.MarshaledRPCIOV)
}

func (rpc *InitializationRPC) GetOutputIOV() sessp.IoVec {
	return make(sessp.IoVec, rpc.GetNOutIOV())
}
