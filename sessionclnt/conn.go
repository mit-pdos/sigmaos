package sessionclnt

import (
	"github.com/sasha-s/go-deadlock"
	"sort"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
	"ulambda/session"
)

// A connection from a client to a logical server (either one server or a
// replica group)
type conn struct {
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

func makeConn(sid np.Tsession, seqno *np.Tseqno, addrs []string) (*conn, *np.Err) {
	c := &conn{}
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

func (c *conn) rpc(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
	if err := c.send(rpc); err != nil {
		return nil, err
	}
	return c.recv(rpc)
}

func (c *conn) send(rpc *netclnt.Rpc) *np.Err {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return np.MkErr(np.TErrUnreachable, c.addrs)
	}
	// Enqueue a request
	c.queue = append(c.queue, rpc)
	c.outstanding[rpc.Req.Seqno] = rpc
	c.Signal()
	return nil
}

func (c *conn) recv(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
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

func (c *conn) connect() *np.Err {
	db.DLPrintf("SESSCONN", "%v Connect to %v\n", c.sid, c.addrs)
	for _, addr := range c.addrs {
		nc, err := netclnt.MakeNetClnt(addr)
		// If this replica is unreachable, try another one.
		if err != nil {
			continue
		}
		db.DLPrintf("SESSCONN", "%v Successful connection to %v out of %v\n", c.sid, addr, c.addrs)
		// If the replica is reachable, save this conn.
		c.nc = nc
		return nil
	}
	db.DLPrintf("SESSCONN", "%v Unable to connect to %v\n", c.sid, c.addrs)
	// No replica is reachable.
	return np.MkErr(np.TErrUnreachable, c.addrs)
}

// If the connection broke, establish a new netclnt connection. If successful,
// resend outstanding requests.
func (c *conn) tryReconnect(oldNc *netclnt.NetClnt) *np.Err {
	c.Lock()
	defer c.Unlock()
	// Check if another thread already reconnected to the replicas.
	if oldNc == c.nc {
		return c.tryReconnectL()
	}
	return nil
}

// Reconnect & resend requests
func (c *conn) tryReconnectL() *np.Err {
	err := c.connect()
	if err != nil {
		db.DLPrintf("SESSCONN", "%v Error %v SessionConn reconnecting to %v\n", c.sid, err, c.addrs)
		return err
	}
	// Resend outstanding requests.
	c.resendOutstanding()
	return nil
}

// Complete an RPC and send a response.
func (c *conn) completeRpc(reply *np.Fcall, err *np.Err) {
	c.Lock()
	rpc, ok := c.outstanding[reply.Seqno]
	delete(c.outstanding, reply.Seqno)
	c.Unlock()
	// the outstanding request map may have been cleared if the conn is closing,
	// in which case rpc will be nil.
	if ok && !rpc.Done {
		db.DLPrintf("SESSCONN", "%v Complete rpc req %v reply %v from %v\n", c.sid, rpc.Req, reply, c.addrs)
		rpc.Done = true
		rpc.ReplyC <- &netclnt.Reply{reply, err}
	}
}

// Caller holds lock.
func (c *conn) resendOutstanding() {
	db.DLPrintf("SESSCONN", "%v Resend outstanding requests to %v\n", c.sid, c.addrs)
	outstanding := make([]*netclnt.Rpc, len(c.outstanding))
	idx := 0
	for _, o := range c.outstanding {
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

func (c *conn) done() bool {
	c.Lock()
	defer c.Unlock()
	return c.closed
}

// Caller holds lock
func (c *conn) close() {
	db.DLPrintf("SESSCONN", "%v Close conn to %v\n", c.sid, c.addrs)
	c.nc.Close()
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

func (c *conn) needsHeartbeat() bool {
	c.Lock()
	defer c.Unlock()
	return time.Now().Sub(c.lastMsgTime) >= session.HEARTBEATMS
}

func (c *conn) heartbeats() {
	for !c.done() {
		// Sleep a bit.
		time.Sleep(session.HEARTBEATMS * time.Millisecond)
		if c.needsHeartbeat() {
			// XXX How soon should I retry if this fails?
			rpc := netclnt.MakeRpc(np.MakeFcall(np.Theartbeat{[]np.Tsession{c.sid}}, c.sid, c.seqno))
			c.send(rpc)
			c.recv(rpc)
		}
	}
}

func (c *conn) reader() {
	for !c.done() {
		// Get the current netclnt connection (which may change if the server
		// becomes unavailable and the writer thread connects to a new replica).
		c.Lock()
		nc := c.nc
		c.Unlock()

		// Receive the next reply.
		reply, err := nc.Recv()
		if err != nil {
			// Try to connect to the next replica
			err := c.tryReconnect(nc)
			if err != nil {
				// If we can't reconnect to any of the replicas, close the session.
				c.Lock()
				c.close()
				c.Unlock()
				return
			}
			// If the connection broke, establish a new netclnt.
			continue
		}
		c.completeRpc(reply, err)
	}
}

func (c *conn) writer() {
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
			db.DLPrintf("SESSCONN", "%v Error %v RPC to %v\n", c.sid, err, c.nc.Dst())
			err := c.tryReconnectL()
			if err != nil {
				c.close()
				return
			}
		}
	}
}
