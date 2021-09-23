package replraft

import (
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protsrv"
)

type Clerk struct {
	mu       *sync.Mutex
	id       int
	fssrv    protsrv.FsServer
	np       protsrv.Protsrv
	opmap    map[np.Tsession]map[np.Tseqno]*SrvOp
	requests chan *SrvOp
	commit   <-chan [][]byte
	proposeC chan<- []byte
}

func makeClerk(id int, fs protsrv.FsServer, commit <-chan [][]byte, propose chan<- []byte) *Clerk {
	c := &Clerk{}
	c.mu = &sync.Mutex{}
	c.id = id
	c.fssrv = fs
	c.np = fs.Connect()
	c.opmap = make(map[np.Tsession]map[np.Tseqno]*SrvOp)
	c.requests = make(chan *SrvOp)
	c.commit = commit
	c.proposeC = propose
	return c
}

func (c *Clerk) request(op *SrvOp) {
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
				req := &np.Fcall{}
				if err := npcodec.Unmarshal(frame, req); err != nil {
					log.Fatalf("Error unmarshalling req in Clerk.serve: %v", err)
				}
				db.DLPrintf("REPLRAFT", "Serve request %v\n", req)
				// XXX Needed to allow watches & locks to progress... but makes things not *quite* correct...
				go func() {
					rep := c.apply(req)
					db.DLPrintf("REPLRAFT", "Reply %v\n", rep)
					// go c.reply(rep)
					c.reply(rep)
				}()
			}
		}
	}
}

func (c *Clerk) propose(op *SrvOp) {
	db.DLPrintf("REPLRAFT", "Propose %v\n", op.request)
	c.registerOp(op)
	c.proposeC <- op.frame
}

func (c *Clerk) apply(fc *np.Fcall) *np.Fcall {
	t := fc.Tag
	// XXX Avoid doing this every time
	c.fssrv.SessionTable().RegisterSession(fc.Session)
	reply, rerror := protsrv.Dispatch(c.np, fc.Session, fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Session = fc.Session
	fcall.Seqno = fc.Seqno
	fcall.Tag = t
	return fcall
}

func (c *Clerk) reply(rep *np.Fcall) {
	op := c.getOp(rep)
	if op == nil {
		return
	}

	op.reply = rep
	op.replyC <- op
}

func (c *Clerk) registerOp(op *SrvOp) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m, ok := c.opmap[op.request.Session]
	if !ok {
		m = make(map[np.Tseqno]*SrvOp)
		c.opmap[op.request.Session] = m
	}
	if _, ok := m[op.request.Seqno]; ok {
		log.Fatalf("Error in Clerk.Propose: seqno already exists")
	}
	m[op.request.Seqno] = op
}

func (c *Clerk) getOp(rep *np.Fcall) *SrvOp {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.opmap[rep.Session]; ok {
		if op, ok := m[rep.Seqno]; ok {
			delete(m, rep.Seqno)
			return op
		}
	}
	return nil
}
