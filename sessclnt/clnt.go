package sessclnt

import (
	"github.com/sasha-s/go-deadlock"
	"sort"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// A session from a client to a logical server (either one server or a
// replica group)
type clnt struct {
	deadlock.Mutex
	*sync.Cond
	sid         np.Tsession
	seqno       *np.Tseqno
	closed      bool
	addrs       []string
	nc          *netclnt.NetClnt
	queue       []*netclnt.Rpc
	outstanding map[np.Tseqno]*netclnt.Rpc // Outstanding requests (which may need to be resent to the next replica if the one we're talking to dies)
	lastMsgTime time.Time
}

func makeConn(sid np.Tsession, seqno *np.Tseqno, addrs []string) (*clnt, *np.Err) {
	c := &clnt{}
	c.sid = sid
	c.seqno = seqno
	c.addrs = addrs
	c.Cond = sync.NewCond(&c.Mutex)
	c.nc = nil
	c.queue = []*netclnt.Rpc{}
	c.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
	err := c.connect()
	if err != nil {
		return nil, err
	}
	go c.reader()
	go c.writer()
	go c.heartbeats()
	return c, nil
}

func (c *clnt) rpc(req np.Tmsg, f np.Tfence) (np.Tmsg, *np.Err) {
	rpc, err := c.send(req, f)
	if err != nil {
		db.DPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", c.sid, req.Type(), req, err, c.addrs)
		return nil, err
	}
	rep, err1 := c.recv(rpc)
	if err1 != nil {
		db.DPrintf("SESSCLNT", "%v Unable to recv response to req %v %v err %v from %v\n", c.sid, req.Type(), req, err, c.addrs)
		return nil, err1
	}
	return rep, err1
}

func (c *clnt) send(req np.Tmsg, f np.Tfence) (*netclnt.Rpc, *np.Err) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, np.MkErr(np.TErrUnreachable, c.addrs)
	}

	rpc := netclnt.MakeRpc(np.MakeFcall(req, c.sid, c.seqno, f))
	// Enqueue a request
	c.queue = append(c.queue, rpc)
	c.outstanding[rpc.Req.Seqno] = rpc
	c.Signal()
	return rpc, nil
}

func (c *clnt) recv(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
	// Wait for a reply
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, np.MkErr(np.TErrUnreachable, c.addrs)
	}
	c.Lock()
	defer c.Unlock()
	c.lastMsgTime = time.Now()
	return reply.Fc.Msg, reply.Err
}

func (c *clnt) connect() *np.Err {
	db.DPrintf("SESSCLNT", "%v Connect to %v\n", c.sid, c.addrs)
	for _, addr := range c.addrs {
		nc, err := netclnt.MakeNetClnt(addr)
		// If this replica is unreachable, try another one.
		if err != nil {
			continue
		}
		db.DPrintf("SESSCLNT", "%v Successful connection to %v out of %v\n", c.sid, addr, c.addrs)
		// If the replica is reachable, save this conn.
		c.nc = nc
		return nil
	}
	db.DPrintf("SESSCLNT", "%v Unable to connect to %v\n", c.sid, c.addrs)
	// No replica is reachable.
	return np.MkErr(np.TErrUnreachable, c.addrs)
}

// If the connection broke, establish a new netclnt connection. If successful,
// resend outstanding requests.
func (c *clnt) tryReconnect(oldNc *netclnt.NetClnt) *np.Err {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return np.MkErr(np.TErrUnreachable, "c closed")
	}
	// Check if another thread already reconnected to the replicas.
	if oldNc == c.nc {
		return c.tryReconnectL()
	}
	return nil
}

// Reconnect & resend requests
func (c *clnt) tryReconnectL() *np.Err {
	db.DPrintf("SESSCLNT", "%v SessionConn reconnecting to %v\n", c.sid, c.addrs)
	err := c.connect()
	if err != nil {
		db.DPrintf("SESSCLNT", "%v Error %v SessionConn reconnecting to %v\n", c.sid, err, c.addrs)
		return err
	}
	// Resend outstanding requests.
	c.resendOutstanding()
	return nil
}

// Complete an RPC and send a response.
func (c *clnt) completeRpc(reply *np.Fcall, err *np.Err) {
	c.Lock()
	rpc, ok := c.outstanding[reply.Seqno]
	delete(c.outstanding, reply.Seqno)
	c.Unlock()
	// the outstanding request map may have been cleared if the conn is closing,
	// in which case rpc will be nil.
	if ok && !rpc.Done {
		db.DPrintf("SESSCLNT", "%v Complete rpc req %v reply %v from %v\n", c.sid, rpc.Req, reply, c.addrs)
		rpc.Done = true
		rpc.ReplyC <- &netclnt.Reply{reply, err}
	}
}

// Caller holds lock.
func (c *clnt) resendOutstanding() {
	db.DPrintf("SESSCLNT", "%v Resend outstanding requests to %v\n", c.sid, c.addrs)
	outstanding := make([]*netclnt.Rpc, len(c.outstanding))
	idx := 0
	for _, o := range c.outstanding {
		db.DPrintf("SESSCLNT0", "%v Resend outstanding requests %v\n", c.sid, o)
		outstanding[idx] = o
		idx++
	}
	sort.Slice(outstanding, func(i, j int) bool {
		return outstanding[i].Req.Seqno < outstanding[j].Req.Seqno
	})
	// Append outstanding requests that need to be resent to the front of the
	// queue.
	c.queue = append(outstanding, c.queue...)
	// Signal that there are queued requests ready to be processed.
	c.Signal()
}

func (c *clnt) isClosed() bool {
	c.Lock()
	defer c.Unlock()
	return c.closed
}

func (c *clnt) sessClose() {
	c.Lock()
	defer c.Unlock()
	c.close()
}

// Caller holds lock
func (c *clnt) close() {
	db.DPrintf("SESSCLNT", "%v Close c to %v\n", c.sid, c.addrs)
	c.nc.Close()
	if c.closed {
		return
	}
	c.closed = true
	// Kill pending requests.
	for _, o := range c.queue {
		if !o.Done {
			o.Done = true
			close(o.ReplyC)
		}
	}
	// Kill outstanding requests.
	for _, o := range c.outstanding {
		if !o.Done {
			o.Done = true
			close(o.ReplyC)
		}
	}
	c.queue = []*netclnt.Rpc{}
	c.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
}

func (c *clnt) needsHeartbeat() bool {
	c.Lock()
	defer c.Unlock()
	return time.Now().Sub(c.lastMsgTime) >= np.SESSHEARTBEATMS
}

func (c *clnt) heartbeats() {
	for !c.isClosed() {
		// Sleep a bit.
		time.Sleep(np.SESSHEARTBEATMS * time.Millisecond)
		if c.needsHeartbeat() {
			// XXX How soon should I retry if this fails?
			db.DPrintf("SESSCLNT", "%v Sending heartbeat to %v", c.sid, c.addrs)
			c.rpc(np.Theartbeat{[]np.Tsession{c.sid}}, np.NoFence)
		}
	}
}

func (c *clnt) reader() {
	for !c.isClosed() {
		// Get the current netclnt connection (which may change if the server
		// becomes unavailable and the writer thread connects to a new replica).
		c.Lock()
		nc := c.nc
		c.Unlock()

		// Receive the next reply.
		reply, err := nc.Recv()
		if err != nil {
			db.DPrintf("SESSCLNT", "%v error %v reader RPC to %v", c.sid, err, c.addrs)
			// Try to connect to the next replica
			err := c.tryReconnect(nc)
			if err != nil {
				// If we can't reconnect to any of the replicas, close the session.
				// XXX fail out standing RPCs
				db.DPrintf("SESSCLNT", "Reader: sessClose %v %v", c.sid, len(c.outstanding))
				c.sessClose()
				return
			}
			// If the connection broke, establish a new netclnt.
			continue
		}
		c.completeRpc(reply, err)
	}
}

func (c *clnt) writer() {
	c.Lock()
	defer c.Unlock()
	for {
		var req *netclnt.Rpc
		// Wait until we have an RPC request.
		for len(c.queue) == 0 {
			if c.closed {
				return
			}
			c.Wait()
		}
		// Pop the first item form the queue.
		req, c.queue = c.queue[0], c.queue[1:]
		err := c.nc.Send(req)
		if err != nil {
			db.DPrintf("SESSCLNT", "%v Error %v writer RPC to %v\n", c.sid, err, c.nc.Dst())
			err := c.tryReconnectL()
			if err != nil {
				// XXX fail out standing req
				db.DPrintf("SESSCLNT", "Writer: c close() %v", c.sid)
				c.close()
				return
			}
		}
	}
}
