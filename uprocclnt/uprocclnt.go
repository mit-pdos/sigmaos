package uprocclnt

import (
	"sigmaos/proc"
	"sigmaos/protdevclnt"
)

type UprocdClnt struct {
	pid proc.Tpid
	*protdevclnt.ProtDevClnt
	shares Tshare
}

func MakeUprocdClnt(pid proc.Tpid, pdc *protdevclnt.ProtDevClnt) *UprocdClnt {
	return &UprocdClnt{
		pid:         pid,
		ProtDevClnt: pdc,
		shares:      0,
	}
}
