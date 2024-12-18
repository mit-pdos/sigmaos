package proto

import (
	sp "sigmaos/sigmap"
)

func (rr *ReplOpReq) Tseqno() sp.Tseqno {
	return sp.Tseqno(rr.Seqno)
}

func (rr *ReplOpReq) TclntId() sp.TclntId {
	return sp.TclntId(rr.ClntId)
}
