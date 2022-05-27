package replies

import (
	"fmt"
	"sort"
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

func (rt *ReplyTable) String() string {
	s := fmt.Sprintf("RT %d: ", len(rt.entries))
	keys := make([]np.Tseqno, 0, len(rt.entries))
	for k, _ := range rt.entries {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return s + "\n"
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	start := keys[0]
	end := keys[0]
	for i := 1; i < len(keys); i++ {
		k := keys[i]
		if k != end+1 {
			s += fmt.Sprintf("[%d,%d) ", start, end)
			start = k
			end = k
		} else {
			end += 1
		}

	}
	s += fmt.Sprintf("[%d,%d)\n", start, end+1)
	return s
}

func (rt *ReplyTable) Register(request *np.Fcall) {
	rt.Lock()
	defer rt.Unlock()

	if rt.closed {
		return
	}
	for s := request.Received.Start; s < request.Received.End; s++ {
		delete(rt.entries, np.Tseqno(s))
	}
	// if seqno in interval tree, then drop
	rt.entries[request.Seqno] = MakeReplyFuture()
}

// Expects that the request has already been registered.
func (rt *ReplyTable) Put(request *np.Fcall, reply *np.Fcall) bool {
	rt.Lock()
	defer rt.Unlock()

	if rt.closed {
		return false
	}
	_, ok := rt.entries[request.Seqno]
	if ok {
		rt.entries[request.Seqno].Complete(reply)
	}
	return ok
}

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
