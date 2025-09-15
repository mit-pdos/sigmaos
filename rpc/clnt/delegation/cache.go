package delegation

import (
	"sync"

	db "sigmaos/debug"
	spproxyproto "sigmaos/proxy/sigmap/proto"
)

// Client-side cache of delegated RPC replies
type ReplyCache struct {
	sync.Mutex
	cond       *sync.Cond
	registered map[uint64]bool
	done       map[uint64]bool
	reps       map[uint64]*spproxyproto.SigmaDelegatedRPCRep
	errors     map[uint64]error
}

func NewReplyCache() *ReplyCache {
	c := &ReplyCache{
		registered: make(map[uint64]bool),
		done:       make(map[uint64]bool),
		reps:       make(map[uint64]*spproxyproto.SigmaDelegatedRPCRep),
		errors:     make(map[uint64]error),
	}
	c.cond = sync.NewCond(&c.Mutex)
	return c
}

// Register an in-progress delegated RPC retrieval
func (c *ReplyCache) Register(idx uint64) {
	c.Lock()
	defer c.Unlock()

	// Sanity check
	if c.registered[idx] {
		db.DFatalf("Double-register delegated RPC(%v)", idx)
	}
	c.registered[idx] = true
	c.done[idx] = false
}

func (c *ReplyCache) Get(idx uint64, rep *spproxyproto.SigmaDelegatedRPCRep) (error, bool) {
	c.Lock()
	defer c.Unlock()
	outiov := rep.Blob.Iov

	// Delegated RPC retrieval is not in-progress, so bail out
	if !c.registered[idx] {
		return nil, false
	}

	// Wait for the delegated RPC retrieval to complete
	for !c.done[idx] {
		c.cond.Wait()
	}
	// Copy reply to outiov
	cachedReply := c.reps[idx]
	// First two entries are the serialized RPC wrappers
	outiov[0] = cachedReply.Blob.Iov[0]
	outiov[1] = cachedReply.Blob.Iov[1]
	// Remaining entries are user-supplied buffers in the RPC Blob, so we need to
	// copy to them
	for i := 2; i < len(outiov); i++ {
		copy(outiov[i], cachedReply.Blob.Iov[i])
	}
	rep.Err = cachedReply.Err
	// Return the result
	return c.errors[idx], true
}

func (c *ReplyCache) Put(idx uint64, rep *spproxyproto.SigmaDelegatedRPCRep, err error) {
	c.Lock()
	defer c.Unlock()

	// Sanity check
	if !c.registered[idx] {
		db.DFatalf("Complete unregistered RPC(%v)", idx)
	}
	// Sanity check
	if c.done[idx] {
		db.DFatalf("Complete already-completed RPC(%v)", idx)
	}
	// Store result
	c.reps[idx] = rep
	c.errors[idx] = err
	c.done[idx] = true
	// Wake up waiters
	c.cond.Broadcast()
}
