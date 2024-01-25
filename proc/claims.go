package proc

import (
	sp "sigmaos/sigmap"
)

func (pc *ProcClaimsProto) GetPID() sp.Tpid {
	return sp.Tpid(pc.PidStr)
}
