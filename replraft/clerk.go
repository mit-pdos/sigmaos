package replraft

import (
	"bytes"
	"log"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
	"sigmaos/threadmgr"
)

type Op struct {
	request   *sessp.FcallMsg
	reply     *sessp.FcallMsg
	frame     []byte
	startTime time.Time
}

type Clerk struct {
	mu       *sync.Mutex
	id       int
	tm       *threadmgr.ThreadMgr
	opmap    map[sessp.Tsession]map[sessp.Tseqno]*Op
	requests chan *Op
	commit   <-chan *committedEntries
	proposeC chan<- []byte
}

func makeClerk(id int, tm *threadmgr.ThreadMgr, commit <-chan *committedEntries, propose chan<- []byte) *Clerk {
	c := &Clerk{}
	c.mu = &sync.Mutex{}
	c.id = id
	c.tm = tm
	c.opmap = make(map[sessp.Tsession]map[sessp.Tseqno]*Op)
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
				if _, req, err := spcodec.ReadUnmarshalFcallAndData(bytes.NewReader(frame)); err != nil {
					db.DFatalf("Error unmarshalling req in Clerk.serve: %v, %v", err, string(frame))
				} else {
					db.DPrintf(db.REPLRAFT, "Serve request %v\n", req)
					//				c.printOpTiming(req, frame)
					c.apply(req, committedReqs.leader)
				}
			}
		}
	}
}

func (c *Clerk) propose(op *Op) {
	db.DPrintf(db.REPLRAFT, "Propose %v\n", op.request)
	op.startTime = time.Now()
	frame, err := spcodec.MarshalFcallAndData(op.request)
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

func (c *Clerk) apply(fc *sessp.FcallMsg, leader uint64) {
	// Get the associated reply channel if this op was generated on this server.
	op := c.getOp(fc)
	if op != nil {
		db.DPrintf(db.RAFT_TIMING, "In-raft op time: %v us %v", time.Now().Sub(op.startTime).Microseconds(), fc)
	}
	// For now, every node can cause a detach to happen
	if fc.GetType() == sessp.TTdetach {
		msg := fc.Msg.(*sp.Tdetach)
		msg.LeadId = uint32(leader)
		fc.Msg = msg
	}
	// Process the op on a single thread.
	c.tm.Process(fc)
}

func (c *Clerk) registerOp(op *Op) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := sessp.Tsession(op.request.Fc.Session)
	seq := op.request.Seqno()
	m, ok := c.opmap[s]
	if !ok {
		m = make(map[sessp.Tseqno]*Op)
		c.opmap[s] = m
	}
	if _, ok := m[seq]; ok {
		// Detaches and server-driven heartbeats may be re-executed many times.
		if op.request.GetType() != sessp.TTdetach && op.request.GetType() != sessp.TTheartbeat {
			db.DFatalf("%v Error in Clerk.Propose: seqno already exists (%v vs %v)", proc.GetName(), op.request, m[seq].request)
		}
	}
	m[seq] = op
}

// Get the full op struct associated with an sessp.
func (c *Clerk) getOp(fc *sessp.FcallMsg) *Op {
	c.mu.Lock()
	defer c.mu.Unlock()

	var op *Op
	s := fc.Session()
	seq := fc.Seqno()
	if m, ok := c.opmap[s]; ok {
		if o, ok := m[seq]; ok {
			delete(m, seq)
			op = o
		}
		if len(m) == 0 {
			delete(c.opmap, s)
		}
	}
	return op
}

// Print how much time an op spent in raft.
func (c *Clerk) printOpTiming(rep *sessp.FcallMsg, frame []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := sessp.Tsession(rep.Fc.Session)
	seqno := rep.Seqno()
	if m, ok := c.opmap[s]; ok {
		if op, ok := m[seqno]; ok {
			log.Printf("In-raft op time: %v us %v bytes", time.Now().Sub(op.startTime).Microseconds(), len(frame))
		}
	}
}
