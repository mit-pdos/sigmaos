package replies

import (
	"fmt"
	"sort"
	"sync"

	"ulambda/intervals"
	np "ulambda/ninep"
)

// Reply table for a given session.
type ReplyTable struct {
	sync.Mutex
	closed   bool
	entries  map[np.Tseqno]*ReplyFuture
	maxSeqno np.Tseqno
	// pruned has seqnos pruned from entries; client has received
	// the response for those.
	pruned *intervals.Intervals
}

func MakeReplyTable() *ReplyTable {
	rt := &ReplyTable{}
	rt.entries = make(map[np.Tseqno]*ReplyFuture)
	rt.pruned = intervals.MkIntervals()
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

func (rt *ReplyTable) Register(request *np.Fcall) bool {
	rt.Lock()
	defer rt.Unlock()

	if rt.closed {
		return false
	}
	for s := request.Received.Start; s < request.Received.End; s++ {
		delete(rt.entries, np.Tseqno(s))
	}
	rt.pruned.Insert(&request.Received)
	// Update the latest seqno we've seen
	if rt.maxSeqno < request.Seqno {
		// If there is a gap, fill the gap.
		//
		// A single seqno counter is shared across all connections to all servers.
		// This is to ensure that a [session, seqno] pair uniquely identifies an RPC
		// in the entire system (which is useful for debuggability). However, this
		// means that the server may see "holes" in the intervals of seqnos it sees
		// from any one client. This can cause its intervals list to grow infinitely
		// long (ReplyTable.pruned).
		//
		// In order to avoid this, we "fill in the gaps" of seqnos which were sent to
		// other servers, and which this server never saw. This should be safe to do,
		// since RPCs are guaranteed to be sent in order. So, if there is a gap
		// between the seqno of this RPC and the last interval this sever knows
		// about, we can safely assume that the intervening RPCs were sent to other
		// servers and "fill in the gap".
		if rt.maxSeqno+1 < request.Seqno {
			rt.pruned.Insert(np.MkInterval(np.Toffset(rt.maxSeqno+1), np.Toffset(request.Seqno)))
		}
		rt.maxSeqno = request.Seqno
	}
	// if seqno in pruned, then drop
	if request.Seqno != 0 && rt.pruned.Contains(np.Toffset(request.Seqno)) {
		return false
	}
	rt.entries[request.Seqno] = MakeReplyFuture()
	return true
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
