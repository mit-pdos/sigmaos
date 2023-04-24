package replies

import (
	"fmt"
	"sort"
	"sync"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

// Reply table for a given session.
type ReplyTable struct {
	sync.Mutex
	cond     *sync.Cond
	sid      sessp.Tsession
	entries  map[sessp.Tseqno]*ReplyFuture
	received []*sessp.Tinterval
	closed   bool
}

func MakeReplyTable(sid sessp.Tsession) *ReplyTable {
	rt := &ReplyTable{}
	rt.cond = sync.NewCond(&rt.Mutex)
	rt.sid = sid
	rt.entries = make(map[sessp.Tseqno]*ReplyFuture)
	rt.received = make([]*sessp.Tinterval, 0, 1000)
	go rt.gc()
	return rt
}

func (rt *ReplyTable) String() string {
	s := fmt.Sprintf("RT %d: ", len(rt.entries))
	keys := make([]sessp.Tseqno, 0, len(rt.entries))
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

func (rt *ReplyTable) Register(request *sessp.FcallMsg) bool {
	rt.Lock()
	defer rt.Unlock()

	if rt.closed {
		return false
	}
	// Marked replies which have been received by the client as ready for GC.
	rt.received = append(rt.received, request.Fc.Received)
	rt.cond.Signal()
	iv := request.Fc.Received
	db.DPrintf(db.REPLY_TABLE, "[%v][%v] Remove seqnos %v", rt.sid, request.Fc.Seqno, iv)
	//	for s := iv.Start; s < iv.End; s++ {
	//		delete(rt.entries, sessp.Tseqno(s))
	//	}
	rt.entries[request.Seqno()] = MakeReplyFuture()
	return true
}

// Expects that the request has already been registered.
func (rt *ReplyTable) Put(request *sessp.FcallMsg, reply *sessp.FcallMsg) bool {
	rt.Lock()
	defer rt.Unlock()

	s := request.Seqno()
	if rt.closed {
		return false
	}
	_, ok := rt.entries[s]
	if ok {
		rt.entries[s].Complete(reply)
	}
	return ok
}

func (rt *ReplyTable) Get(request *sessp.Fcall) (*ReplyFuture, bool) {
	rt.Lock()
	defer rt.Unlock()
	rf, ok := rt.entries[request.Tseqno()]
	return rf, ok
}

// Empty and permanently close the replies table. There may be server-side
// threads waiting on reply results, so make sure to complete all of them with
// an error.
func (rt *ReplyTable) Close(cli sessp.Tclient, sid sessp.Tsession) {
	rt.Lock()
	defer rt.Unlock()
	for _, rf := range rt.entries {
		rf.Abort(cli, sid)
	}
	rt.entries = make(map[sessp.Tseqno]*ReplyFuture)
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

// Garbage-collect received replies.
func (rt *ReplyTable) gc() {
	rt.Lock()
	defer rt.Unlock()

	for !rt.closed {
		// Wait for there to be replies to garbage-collect.
		for len(rt.received) == 0 {
			rt.cond.Wait()
		}
		// Pop the last interval.
		iv := rt.received[len(rt.received)-1]
		rt.received = rt.received[:len(rt.received)-1]
		// Unlock, so other threads can make progress.
		rt.Unlock()
		// Remove stored replies which the client has already received. The reply is
		// always expected to be present, unless there has been a partition and the
		// client has to resend some RPCs.
		for s := iv.Start; s < iv.End; s++ {
			rt.Lock()
			delete(rt.entries, sessp.Tseqno(s))
			rt.Unlock()
		}
		// Lock for cond.Wait call.
		rt.Lock()
	}
}
