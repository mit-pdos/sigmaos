package proc

import (
	sp "sigmaos/sigmap"
)

func (pc *ProcClaimsProto) GetRealm() sp.Trealm {
	return sp.Trealm(pc.RealmStr)
}

func (pc *ProcClaimsProto) GetPrincipalID() sp.TprincipalID {
	return sp.TprincipalID(pc.PrincipalIDStr)
}
