package replraft

import (
	"log"
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/proc"
	"ulambda/threadmgr"
)

type Op struct {
	request   *np.Fcall
	reply     *np.Fcall
	startTime time.Time
}

type Clerk struct {
	mu       *sync.Mutex
	id       int
	tm       *threadmgr.ThreadMgr
	opmap    map[np.Tsession]map[np.Tseqno]*Op
	requests chan *Op
	commit   <-chan [][]byte
	proposeC chan<- []byte
}

func makeClerk(id int, tm *threadmgr.ThreadMgr, commit <-chan [][]byte, propose chan<- []byte) *Clerk {
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
			for _, frame := range committedReqs {
				if req, err := npcodec.UnmarshalFcall(frame); err != nil {
					db.DFatalf("FATAL Error unmarshalling req in Clerk.serve: %v, %v", err, string(frame))
				} else {
					db.DPrintf("REPLRAFT", "Serve request %v\n", req)
					// XXX Needed to allow watches & locks to progress... but makes things not *quite* correct...
					//				c.printOpTiming(req, frame)
					c.apply(req)
				}
			}
		}
	}
}

func (c *Clerk) propose(op *Op) {
	db.DPrintf("REPLRAFT", "Propose %v\n", op.request)
	op.startTime = time.Now()
	c.registerOp(op)
	frame, err := npcodec.MarshalFcallByte(op.request)
	if err != nil {
		db.DFatalf("FATAL: marshal op in replraft.Clerk.Propose: %v", err)
	}
	c.proposeC <- frame
}

func (c *Clerk) apply(fc *np.Fcall) {
	// Get the associated reply channel if this op was generated on this server.
	c.getOp(fc)
	// For now, every node can cause a detach to happen
	if fc.GetType() == np.TTdetach {
		msg := fc.Msg.(np.Tdetach)
		msg.LeadId = msg.PropId
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
		// Detaches may be re-executed many times.
		if op.request.GetType() != np.TTdetach {
			db.DFatalf("FATAL %v Error in Clerk.Propose: seqno already exists (%v vs %v)", proc.GetName(), op.request, m[op.request.Seqno].request)
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
