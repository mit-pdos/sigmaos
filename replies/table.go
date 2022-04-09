package replies

import (
	"sync"

	np "ulambda/ninep"
)

// Reply table for a given session.
type ReplyTable struct {
	sync.Mutex
	closed  bool
	entries map[np.Tseqno]*ReplyFuture
}

func MakeReplyTable() *ReplyTable {
	rt := &ReplyTable{}
	rt.entries = make(map[np.Tseqno]*ReplyFuture)
	return rt
}

func (rt *ReplyTable) Register(request *np.Fcall) {
	rt.Lock()
	defer rt.Unlock()

	if rt.closed {
		return
	}
	rt.entries[request.Seqno] = MakeReplyFuture()
}

// Expects that the request has already been registered.
func (rt *ReplyTable) Put(request *np.Fcall, reply *np.Fcall) {
	rt.Lock()
	defer rt.Unlock()

	if rt.closed {
		return
	}
	rt.entries[request.Seqno].Complete(reply)
}

// XXX Will need to handle entries which are "too old" eventually once we
// start evicting entries from the cache.
func (rt *ReplyTable) Get(request *np.Fcall) (*ReplyFuture, bool) {
	rt.Lock()
	defer rt.Unlock()
	rf, ok := rt.entries[request.Seqno]
	return rf, ok
}

// Empty and permanently close the replies table. There may be server-side
// threads waiting on reply results, so make sure to complete all of them with
// an error.
func (rt *ReplyTable) Close(sid np.Tsession) {
	rt.Lock()
	defer rt.Unlock()
	for _, rf := range rt.entries {
		rf.Abort(sid)
	}
	rt.entries = make(map[np.Tseqno]*ReplyFuture)
	rt.closed = true
}

// Merge two reply caches.
func (rt *ReplyTable) Merge(rt2 *ReplyTable) {
	for seqno, entry := range rt2.entries {
		rf := MakeReplyFuture()
		if entry.reply != nil {
			rf.Complete(entry.reply)
		}
		rt.entries[seqno] = rf
	}
}
