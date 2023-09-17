package replies

import (
	"sync"

	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

// Future for a reply.
type ReplyFuture struct {
	sync.Mutex
	*sync.Cond
	reply *sessp.FcallMsg
}

func NewReplyFuture() *ReplyFuture {
	r := &ReplyFuture{}
	r.Cond = sync.NewCond(&r.Mutex)
	return r
}

// Wait for a reply.
func (f *ReplyFuture) Await() *sessp.FcallMsg {
	f.Lock()
	defer f.Unlock()
	// Potentially wait for a blocked op to complete.
	if f.Cond != nil {
		f.Wait()
	}
	return f.reply
}

// Wake waiters for a reply.
func (f *ReplyFuture) Complete(fc *sessp.FcallMsg) {
	f.Lock()
	defer f.Unlock()
	f.reply = fc
	// Mark that a reply has been received, so no one tries to wait in the
	// future.
	if f.Cond != nil {
		f.Cond.Broadcast()
		f.Cond = nil
	}
}

// Abort waiting for a reply.
func (f *ReplyFuture) Abort(cli sessp.Tclient, sid sessp.Tsession) {
	f.Lock()
	defer f.Unlock()
	if f.Cond != nil {
		f.reply = sessp.NewFcallMsg(sp.MkRerror(serr.MkErr(serr.TErrClosed, nil)), nil, cli, sid, nil, sessp.Tinterval{})
		f.Cond.Broadcast()
		f.Cond = nil
	}
}
