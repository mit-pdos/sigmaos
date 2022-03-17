package replraft

import (
	"log"
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/threadmgr"
)

type Op struct {
	request   *np.Fcall
	reply     *np.Fcall
	replyC    chan *np.Fcall
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
					log.Fatalf("FATAL Error unmarshalling req in Clerk.serve: %v, %v", err, string(frame))
				} else {
					db.DLPrintf("REPLRAFT", "Serve request %v\n", req)
					// XXX Needed to allow watches & locks to progress... but makes things not *quite* correct...
					//				c.printOpTiming(req, frame)
					c.apply(req)
				}
			}
		}
	}
}

func (c *Clerk) propose(op *Op) {
	db.DLPrintf("REPLRAFT", "Propose %v\n", op.request)
	op.startTime = time.Now()
	c.registerOp(op)
	frame, err := npcodec.MarshalFcallByte(op.request)
	if err != nil {
		log.Fatalf("FATAL: marshal op in replraft.Clerk.Propose: %v", err)
	}
	c.proposeC <- frame
}

func (c *Clerk) apply(fc *np.Fcall) {
	var replies chan *np.Fcall = nil
	// Get the associated reply channel if this op was generated on this server.
	op := c.getOp(fc)
	if op != nil {
		replies = op.replyC
	}
	// For now, every node can cause a detach to happen
	if fc.GetType() == np.TTdetach {
		fc.GetMsg().(*np.Tdetach).LeadId = fc.GetMsg().(*np.Tdetach).PropId
	}
	// Process the op on a single thread.
	c.tm.Process(fc, replies)
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
		log.Fatalf("Error in Clerk.Propose: seqno already exists")
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
