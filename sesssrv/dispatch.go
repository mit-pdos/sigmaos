package sesssrv

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sigmaprotsrv"
)

func (s *Session) Dispatch(msg sessp.Tmsg, iov sessp.IoVec) (sessp.Tmsg, sessp.IoVec, *sp.Rerror, sigmaprotsrv.Tsessop, sp.TclntId) {
	if s.IsClosed() {
		db.DPrintf(db.SESS_STATE_SRV, "Sess %v is closed; reject %v\n", s.Sid, msg.Type())
		err := serr.NewErr(serr.TErrClosed, fmt.Sprintf("session %v", s.Sid))
		return nil, nil, sp.NewRerrorSerr(err), sigmaprotsrv.TSESS_NONE, sp.NoClntId
	}
	return sigmaprotsrv.Dispatch(s.protsrv, msg, iov)
}
