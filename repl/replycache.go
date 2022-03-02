package repl

import (
	"sync"

	np "ulambda/ninep"
)

type ReplyFuture struct {
	sync.Mutex
	*sync.Cond
	reply np.Tmsg
}

func MakeReplyFuture() *ReplyFuture {
	r := &ReplyFuture{}
	r.Cond = sync.NewCond(&r.Mutex)
	return r
}

// Wait for a reply.
func (f *ReplyFuture) Await() np.Tmsg {
	f.Lock()
	defer f.Unlock()
	// Potentially wait for a blocked op to complete.
	if f.Cond != nil {
		f.Wait()
	}
	return f.reply
}

// Wake waiters for a reply.
func (f *ReplyFuture) Complete(msg np.Tmsg) {
	f.Lock()
	defer f.Unlock()
	f.reply = msg
	// Mark that a reply has been received, so no one tries to wait in the
	// future.
	if f.Cond != nil {
		f.Cond.Broadcast()
		f.Cond = nil
	}
}

type ReplyCache struct {
	sync.Mutex
	entries map[np.Tsession]map[np.Tseqno]*ReplyFuture
}

func MakeReplyCache() *ReplyCache {
	rc := &ReplyCache{}
	rc.entries = map[np.Tsession]map[np.Tseqno]*ReplyFuture{}
	return rc
}

func (rc *ReplyCache) Register(request *np.Fcall) {
	rc.Lock()
	defer rc.Unlock()
	if _, ok := rc.entries[request.Session]; !ok {
		rc.entries[request.Session] = map[np.Tseqno]*ReplyFuture{}
	}
	rc.entries[request.Session][request.Seqno] = MakeReplyFuture()
}

// Expects that the request has already been registered.
func (rc *ReplyCache) Put(request *np.Fcall, reply np.Tmsg) {
	rc.Lock()
	defer rc.Unlock()
	rc.entries[request.Session][request.Seqno].Complete(reply)
}

// XXX Will need to handle entries which are "too old" eventually once we
// start evicting entries from the cache.
func (rc *ReplyCache) Get(request *np.Fcall) (*ReplyFuture, bool) {
	rc.Lock()
	defer rc.Unlock()
	if sessionMap, ok := rc.entries[request.Session]; !ok {
		return nil, false
	} else {
		rf, ok := sessionMap[request.Seqno]
		return rf, ok
	}
}
