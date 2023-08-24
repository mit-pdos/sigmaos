package replraft

import (
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/repl"
	replproto "sigmaos/repl/proto"
	sp "sigmaos/sigmap"
)

type Op struct {
	request   *replproto.ReplOpRequest
	reply     *replproto.ReplOpReply
	err       error
	startTime time.Time
	ch        chan struct{}
}

type Clerk struct {
	mu       *sync.Mutex
	opmap    map[sp.TclntId]map[sp.Tseqno]*Op
	requests chan *Op
	commit   <-chan *committedEntries
	proposeC chan<- []byte
	applyf   repl.Tapplyf
}

func newClerk(commit <-chan *committedEntries, propose chan<- []byte, applyf repl.Tapplyf) *Clerk {
	return &Clerk{
		mu:       &sync.Mutex{},
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
					// c.printOpTiming(req, frame)
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
	c.registerOp(op.request, op)
	c.proposeC <- frame
}

func (c *Clerk) apply(req *replproto.ReplOpRequest, leader uint64) {
	op := c.getOp(req)
	if op != nil { // let proposer know its message has been applied
		op.err = c.applyf(req, op.reply)
		op.ch <- struct{}{}
	} else {
		c.applyf(req, nil)
	}
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
