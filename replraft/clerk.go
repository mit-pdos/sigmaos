package replraft

import (
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
	replproto "sigmaos/repl/proto"
	rpcproto "sigmaos/rpc/proto"
	sp "sigmaos/sigmap"
)

type Op struct {
	request   *replproto.ReplRequest
	clntId    sp.TclntId
	seqno     sp.Tseqno
	reply     *rpcproto.Reply
	frame     []byte
	startTime time.Time
}

type Clerk struct {
	mu       *sync.Mutex
	id       int
	opmap    map[sp.TclntId]map[sp.Tseqno]*Op
	requests chan *Op
	commit   <-chan *committedEntries
	proposeC chan<- []byte
}

func makeClerk(id int, commit <-chan *committedEntries, propose chan<- []byte) *Clerk {
	c := &Clerk{}
	c.mu = &sync.Mutex{}
	c.id = id
	c.opmap = make(map[sp.TclntId]map[sp.Tseqno]*Op)
	c.requests = make(chan *Op)
	c.commit = commit
	c.proposeC = propose
	return c
}

func (c *Clerk) request(op *Op) {
	c.requests <- op
}

func (c *Clerk) serve() {
	for {
		// TODO: re-propose ops on a ticker
		select {
		case req := <-c.requests:
			go c.propose(req)
		case committedReqs := <-c.commit:
			for _, frame := range committedReqs.entries {
				req := replproto.ReplRequest{}
				if err := proto.Unmarshal(frame, &req); err != nil {
					db.DFatalf("Error unmarshalling req in Clerk.serve: %v, %v", err, string(frame))
				} else {
					db.DPrintf(db.REPLRAFT, "Serve request %v\n", req)
					//				c.printOpTiming(req, frame)
					c.apply(&req, committedReqs.leader)
				}
			}
		}
	}
}

func (c *Clerk) propose(op *Op) {
	db.DPrintf(db.REPLRAFT, "Propose %v\n", op.request)
	op.startTime = time.Now()
	frame, err := proto.Marshal(op.request)
	if err != nil {
		db.DFatalf("marshal op in replraft.Clerk.Propose: %v", err)
	}
	op.frame = frame
	c.registerOp(op)
	c.proposeC <- frame
}

// Repropose pending ops, in the event that leadership may have changed.
func (c *Clerk) reproposeOps() {
	c.mu.Lock()
	frames := [][]byte{}
	for _, m := range c.opmap {
		for _, op := range m {
			frames = append(frames, op.frame)
		}
	}
	c.mu.Unlock()
	for _, f := range frames {
		c.proposeC <- f
	}
}

func (c *Clerk) apply(req *replproto.ReplRequest, leader uint64) {
	// Get the associated reply channel if this op was generated on this server.
	op := c.getOp(req)
	if op != nil {
		db.DPrintf(db.RAFT_TIMING, "In-raft op time: %v us %v", time.Now().Sub(op.startTime).Microseconds(), req)
	}
	// Process the op on a single thread.
	// Apply op to cache
	// XXX c.tm.Process(fc)
}

func (c *Clerk) registerOp(op *Op) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cid := op.clntId
	seq := op.seqno
	m, ok := c.opmap[cid]
	if !ok {
		m = make(map[sp.Tseqno]*Op)
		c.opmap[cid] = m
	}
	if _, ok := m[seq]; ok {
		db.DFatalf("%v Error in Clerk.Propose: seqno already exists (%v vs %v)", proc.GetName(), op.request, m[seq].request)
	}
	m[seq] = op
}

// Get the full op struct associated with an sp.
func (c *Clerk) getOp(req *replproto.ReplRequest) *Op {
	c.mu.Lock()
	defer c.mu.Unlock()

	var op *Op
	cid := req.TclntId()
	seq := req.Tseqno()
	if m, ok := c.opmap[cid]; ok {
		if o, ok := m[seq]; ok {
			delete(m, seq)
			op = o
		}
		if len(m) == 0 {
			delete(c.opmap, cid)
		}
	}
	return op
}

// Print how much time an op spent in raft.
// func (c *Clerk) printOpTiming(rep *sessp.FcallMsg, frame []byte) {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	s := rep.ClntId()
// 	seqno := rep.Seqno()
// 	if m, ok := c.opmap[s]; ok {
// 		if op, ok := m[seqno]; ok {
// 			log.Printf("In-raft op time: %v us %v bytes", time.Now().Sub(op.startTime).Microseconds(), len(frame))
// 		}
// 	}
// }
