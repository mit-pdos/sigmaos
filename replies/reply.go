package replies

import (
	"sync"

	np "sigmaos/ninep"
)

// Future for a reply.
type ReplyFuture struct {
	sync.Mutex
	*sync.Cond
	reply *np.FcallMsg
}

func MakeReplyFuture() *ReplyFuture {
	r := &ReplyFuture{}
	r.Cond = sync.NewCond(&r.Mutex)
	return r
}

// Wait for a reply.
func (f *ReplyFuture) Await() *np.FcallMsg {
	f.Lock()
	defer f.Unlock()
	// Potentially wait for a blocked op to complete.
	if f.Cond != nil {
		f.Wait()
	}
	return f.reply
}

// Wake waiters for a reply.
func (f *ReplyFuture) Complete(fc *np.FcallMsg) {
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
func (f *ReplyFuture) Abort(cli np.Tclient, sid np.Tsession) {
	f.Lock()
	defer f.Unlock()
	if f.Cond != nil {
		f.reply = np.MakeFcallMsg(np.MkErr(np.TErrClosed, nil).Rerror(), cli, sid, nil, nil, np.MakeFenceNull())
		f.Cond.Broadcast()
		f.Cond = nil
	}
}
