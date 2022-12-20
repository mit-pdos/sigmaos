package replies

import (
	"sync"

	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

// Future for a reply.
type ReplyFuture struct {
	sync.Mutex
	*sync.Cond
	reply *fcall.FcallMsg
}

func MakeReplyFuture() *ReplyFuture {
	r := &ReplyFuture{}
	r.Cond = sync.NewCond(&r.Mutex)
	return r
}

// Wait for a reply.
func (f *ReplyFuture) Await() *fcall.FcallMsg {
	f.Lock()
	defer f.Unlock()
	// Potentially wait for a blocked op to complete.
	if f.Cond != nil {
		f.Wait()
	}
	return f.reply
}

// Wake waiters for a reply.
func (f *ReplyFuture) Complete(fc *fcall.FcallMsg) {
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
func (f *ReplyFuture) Abort(cli fcall.Tclient, sid fcall.Tsession) {
	f.Lock()
	defer f.Unlock()
	if f.Cond != nil {
		f.reply = fcall.MakeFcallMsg(sp.MkRerror(fcall.MkErr(fcall.TErrClosed, nil)), nil, cli, sid, nil, nil, fcall.MakeFenceNull())
		f.Cond.Broadcast()
		f.Cond = nil
	}
}
