package replraft

import (
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	replproto "sigmaos/cache/replproto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/repl"
	sp "sigmaos/sigmap"
)

type Op struct {
	request   *replproto.ReplOpRequest
	reply     *replproto.ReplOpReply
	frame     []byte
	startTime time.Time
	ch        chan error
}

type Clerk struct {
	mu       *sync.Mutex
	id       int
	opmap    map[sp.TclntId]map[sp.Tseqno]*Op
	requests chan *Op
	commit   <-chan *committedEntries
	proposeC chan<- []byte
	applyf   repl.Tapplyf
}

func newClerk(id int, commit <-chan *committedEntries, propose chan<- []byte, applyf repl.Tapplyf) *Clerk {
	return &Clerk{
		mu:       &sync.Mutex{},
		id:       id,
		opmap:    make(map[sp.TclntId]map[sp.Tseqno]*Op),
		applyf:   applyf,
		requests: make(chan *Op),
		commit:   commit,
		proposeC: propose,
	}
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
				req := replproto.ReplOpRequest{}
				if err := proto.Unmarshal(frame, &req); err != nil {
					db.DFatalf("Error unmarshalling req in Clerk.serve: %v, %v", err, string(frame))
				} else {
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
	c.registerOp(op.request, op)
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

func (c *Clerk) apply(req *replproto.ReplOpRequest, leader uint64) {
	op := c.getOp(req)
	if op == nil {
		db.DFatalf("no op %v\n", req)
	}
	db.DPrintf(db.REPLRAFT, "Serve request %v %v\n", req, op)
	err := c.applyf(req, op.reply)
	op.ch <- err
}

func (c *Clerk) registerOp(req *replproto.ReplOpRequest, op *Op) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cid := req.TclntId()
	seq := req.Tseqno()
	m, ok := c.opmap[cid]
	if !ok {
		m = make(map[sp.Tseqno]*Op)
		c.opmap[cid] = m
	}
	if _, ok := m[seq]; ok {
		db.DFatalf("%v registerOp (%v vs %v)", proc.GetName(), op.request, m[seq].request)
	}
	m[seq] = op
}

// Get the full op struct associated with an sp.
func (c *Clerk) getOp(req *replproto.ReplOpRequest) *Op {
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
