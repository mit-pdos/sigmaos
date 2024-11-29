package replsrv

import (
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

//
// Reply table for a given client
//

type Tentry struct {
	seqno sp.Tseqno
	err   error
	reply []byte
}

type Treplies map[sp.TclntId]*Tentry

type ReplyTable struct {
	sync.Mutex
	cid     sp.TclntId
	replies Treplies
}

func NewReplyTable() *ReplyTable {
	rt := &ReplyTable{}
	rt.replies = make(Treplies)
	return rt
}

func (rt *ReplyTable) IsDuplicate(cid sp.TclntId, seqno sp.Tseqno) (bool, error, []byte) {
	rt.Lock()
	defer rt.Unlock()

	e, ok := rt.replies[cid]
	if ok && e.seqno >= seqno {
		return true, e.err, e.reply
	}
	return false, nil, nil
}

func (rt *ReplyTable) PutReply(cid sp.TclntId, seqno sp.Tseqno, err error, b []byte) {
	rt.Lock()
	defer rt.Unlock()

	e, ok := rt.replies[cid]
	if ok {
		if e.seqno >= seqno {
			db.DFatalf("overwriting new or duplicte %v %v\n", cid, seqno)
		}
	} else {
		e = &Tentry{}
		rt.replies[cid] = e
	}
	e.seqno = seqno
	e.reply = b
	e.err = err
}
