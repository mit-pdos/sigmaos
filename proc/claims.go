package proc

import (
	sp "sigmaos/sigmap"
)

func (pc *ProcClaimsProto) GetPrincipalID() sp.TprincipalID {
	return sp.TprincipalID(pc.PrincipalIDStr)
}
