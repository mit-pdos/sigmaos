package replraft

import (
	"log"
	"sync"
	"time"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/npcodec"
	"sigmaos/proc"
	"sigmaos/threadmgr"
)

type Op struct {
	request   *np.Fcall
	reply     *np.Fcall
	frame     []byte
	startTime time.Time
}

type Clerk struct {
	mu       *sync.Mutex
	id       int
	tm       *threadmgr.ThreadMgr
	opmap    map[np.Tsession]map[np.Tseqno]*Op
	requests chan *Op
	commit   <-chan *committedEntries
	proposeC chan<- []byte
}

func makeClerk(id int, tm *threadmgr.ThreadMgr, commit <-chan *committedEntries, propose chan<- []byte) *Clerk {
	c := &Clerk{}
	c.mu = &sync.Mutex{}
	c.id = id
	c.tm = tm
	c.opmap = make(map[np.Tsession]map[np.Tseqno]*Op)
	c.requests = make(chan *Op)
	c.commit = commit
	c.proposeC = propose
	return c
}

// Put above process
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
				if req, err := npcodec.UnmarshalFcall(frame); err != nil {
					db.DFatalf("Error unmarshalling req in Clerk.serve: %v, %v", err, string(frame))
				} else {
					db.DPrintf("REPLRAFT", "Serve request %v\n", req)
					//				c.printOpTiming(req, frame)
					c.apply(req, committedReqs.leader)
				}
			}
		}
	}
}

func (c *Clerk) propose(op *Op) {
	db.DPrintf("REPLRAFT", "Propose %v\n", op.request)
	op.startTime = time.Now()
	frame, err := npcodec.MarshalFcallByte(op.request)
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

func (c *Clerk) apply(fc *np.Fcall, leader uint64) {
	// Get the associated reply channel if this op was generated on this server.
	op := c.getOp(fc)
	if op != nil {
		db.DPrintf("TIMING", "In-raft op time: %v us %v", time.Now().Sub(op.startTime).Microseconds(), fc)
	}
	// For now, every node can cause a detach to happen
	if fc.GetType() == np.TTdetach {
		msg := fc.Msg.(*np.Tdetach)
		msg.LeadId = uint32(leader)
		fc.Msg = msg
	}
	// Process the op on a single thread.
	c.tm.Process(fc)
}

func (c *Clerk) registerOp(op *Op) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m, ok := c.opmap[op.request.Session]
	if !ok {
		m = make(map[np.Tseqno]*Op)
		c.opmap[op.request.Session] = m
	}
	if _, ok := m[op.request.Seqno]; ok {
		// Detaches and server-driven heartbeats may be re-executed many times.
		if op.request.GetType() != np.TTdetach && op.request.GetType() != np.TTheartbeat {
			db.DFatalf("%v Error in Clerk.Propose: seqno already exists (%v vs %v)", proc.GetName(), op.request, m[op.request.Seqno].request)
		}
	}
	m[op.request.Seqno] = op
}

// Get the full op struct associated with an fcall.
func (c *Clerk) getOp(fc *np.Fcall) *Op {
	c.mu.Lock()
	defer c.mu.Unlock()

	var op *Op
	if m, ok := c.opmap[fc.Session]; ok {
		if o, ok := m[fc.Seqno]; ok {
			delete(m, fc.Seqno)
			op = o
		}
		if len(m) == 0 {
			delete(c.opmap, fc.Session)
		}
	}
	return op
}

// Print how much time an op spent in raft.
func (c *Clerk) printOpTiming(rep *np.Fcall, frame []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.opmap[rep.Session]; ok {
		if op, ok := m[rep.Seqno]; ok {
			log.Printf("In-raft op time: %v us %v bytes", time.Now().Sub(op.startTime).Microseconds(), len(frame))
		}
	}
}
