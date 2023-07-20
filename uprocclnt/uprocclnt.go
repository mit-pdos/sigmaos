package uprocclnt

import (
	"fmt"

	"sigmaos/proc"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type UprocdClnt struct {
	pid proc.Tpid
	*rpcclnt.RPCClnt
	realm sp.Trealm
	ptype proc.Ttype
	share Tshare
}

func MakeUprocdClnt(pid proc.Tpid, pdc *rpcclnt.RPCClnt, realm sp.Trealm, ptype proc.Ttype) *UprocdClnt {
	return &UprocdClnt{
		pid:         pid,
		RPCClnt: pdc,
		realm:       realm,
		ptype:       ptype,
		share:       0,
	}
}

func (clnt *UprocdClnt) String() string {
	return fmt.Sprintf("&{ realm:%v ptype:%v share:%v }", clnt.realm, clnt.ptype, clnt.share)
}
