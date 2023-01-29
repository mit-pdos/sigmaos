package uprocclnt

import (
	"fmt"

	"sigmaos/proc"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
)

type UprocdClnt struct {
	pid proc.Tpid
	*protdevclnt.ProtDevClnt
	realm  sp.Trealm
	ptype  proc.Ttype
	shares Tshare
}

func MakeUprocdClnt(pid proc.Tpid, pdc *protdevclnt.ProtDevClnt, realm sp.Trealm, ptype proc.Ttype) *UprocdClnt {
	return &UprocdClnt{
		pid:         pid,
		ProtDevClnt: pdc,
		realm:       realm,
		ptype:       ptype,
		shares:      0,
	}
}

func (clnt *UprocdClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v shares:%v }", clnt.realm, clnt.ptype, clnt.shares)
}
