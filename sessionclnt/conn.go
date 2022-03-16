package sessionclnt

import (
	"sort"
	"sync"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// A connection from a client to a logical server (either one server or a
// replica group)
type conn struct {
	sync.Mutex
	*sync.Cond
	closed      bool
	addrs       []string
	nc          *netclnt.NetClnt
	queue       []*netclnt.Rpc
	outstanding map[np.Tseqno]*netclnt.Rpc // Outstanding requests (which may need to be resent to the next replica if the one we're talking to dies)
}

func makeConn(addrs []string) (*conn, *np.Err) {
	c := &conn{}
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
	return reply.Fc.Msg, reply.Err
}

func (c *conn) connect() *np.Err {
	for _, addr := range c.addrs {
		nc, err := netclnt.MakeNetClnt(addr)
		// If this replica is unreachable, try another one.
		if err != nil {
			continue
		}
		// If the replica is reachable, save this conn.
		c.nc = nc
		return nil
	}
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
		db.DLPrintf("SESSCONN", "Error %v SessionConn reconnecting to %v\n", err, c.addrs)
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
		rpc.Done = true
		rpc.ReplyC <- &netclnt.Reply{reply, err}
	}
}

func (c *conn) reader() {
	for {
		// Get the current netclnt connection (which may change if the server
		// becomes unavailable and the writer thread connects to a new replica).
		c.Lock()
		closed := c.closed
		c.Unlock()

		if closed {
			return
		}
		nc := c.nc

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
			db.DLPrintf("SESSCONN", "Error %v RPC to %v\n", err, c.nc.Dst())
			err := c.tryReconnectL()
			if err != nil {
				c.close()
				return
			}
		}
	}
}

// Caller holds lock.
func (c *conn) resendOutstanding() {
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

func (c *conn) close() {
	c.nc.Close()
	c.closed = true
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
