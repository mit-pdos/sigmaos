package proto

import (
	sp "sigmaos/sigmap"
)

func (rr *ReplOpRequest) Tseqno() sp.Tseqno {
	return sp.Tseqno(rr.Seqno)
}

func (rr *ReplOpRequest) TclntId() sp.TclntId {
	return sp.TclntId(rr.ClntId)
}
