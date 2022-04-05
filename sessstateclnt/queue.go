package sessstateclnt

import (
	"sort"
	"sync"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// A Request Queue which guarantees:
//
// 1. Enqueued requests are always returned in order of sequence number between
//    resets.
// 2. A request will only successfully be removed once.
// 3. If a request hasn't been removed, and the request queue is reset, then
//    the request will be enqueued again in order of sequence number.
// 4. Once closed, a request queue will refuse (panic on) future request
//    enqueue attempts.
// 5. Close removes all outstanding requests.

type RequestQueue struct {
	sync.Mutex
	*sync.Cond
	queue       []*netclnt.Rpc
	outstanding map[np.Tseqno]*netclnt.Rpc // Outstanding requests (which may need to be resent to the next replica if the one we're talking to dies)
	closed      bool
}

func MakeRequestQueue() *RequestQueue {
	rq := &RequestQueue{}
	rq.Cond = sync.NewCond(&rq.Mutex)
	rq.queue = []*netclnt.Rpc{}
	rq.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
	return rq
}

// Add a new request to the queue.
func (rq *RequestQueue) Enqueue(rpc *netclnt.Rpc) {
	rq.Lock()
	defer rq.Unlock()

	if rq.closed {
		db.DFatalf("Tried to enqueue a request on a closed request queue %v", rpc.Req)
	}

	if _, ok := rq.outstanding[rpc.Req.Seqno]; ok {
		db.DFatalf("Tried to enqueue a duplicate request %v", rpc.Req)
	}
	rq.queue = append(rq.queue, rpc)
	rq.outstanding[rpc.Req.Seqno] = rpc
	rq.Signal()
}

// Get the next request to be processed, in order of sequence numbers.
func (rq *RequestQueue) Next() *netclnt.Rpc {
	rq.Lock()
	defer rq.Unlock()
	var req *netclnt.Rpc
	// Wait until we have an RPC request which needs to be processed.
	for len(rq.queue) == 0 {
		if rq.closed {
			return nil
		}
		rq.Wait()
	}
	// Pop the first item form the queue.
	req, rq.queue = rq.queue[0], rq.queue[1:]
	return req
}

// Remove a request and return it. If it doesn't exist in the
// request queue, someone else has removed it already, so we return
// nil & false.
func (rq *RequestQueue) Remove(seqno np.Tseqno) (*netclnt.Rpc, bool) {
	rq.Lock()
	defer rq.Unlock()

	if rpc, ok := rq.outstanding[seqno]; ok {
		delete(rq.outstanding, seqno)
		return rpc, true
	}
	return nil, false
}

// Reset the request queue to contain all outstanding requests, in order. This
// should be called immediately upon reconnect.
func (rq *RequestQueue) Reset() {
	rq.Lock()
	defer rq.Unlock()

	new := make([]*netclnt.Rpc, len(rq.outstanding))
	idx := 0
	for _, o := range rq.outstanding {
		new[idx] = o
		idx++
	}
	sort.Slice(new, func(i, j int) bool {
		return new[i].Req.Seqno < new[j].Req.Seqno
	})
	rq.queue = new
	// Signal that there are queued requests ready to be processed.
	rq.Signal()
}

// Close the request queue, and return any outstanding requests
func (rq *RequestQueue) Close() map[np.Tseqno]*netclnt.Rpc {
	rq.Lock()
	defer rq.Unlock()

	// Mark the request queue as closed
	rq.closed = true
	// Save the old map of outstanding requests, and return it
	o := rq.outstanding
	// Empty the map of outstanding requests and the request queue.
	rq.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
	rq.queue = []*netclnt.Rpc{}
	return o
}
