package sessionclnt

import (
	"log"
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
	go c.reader()
	go c.writer()
	return c, c.connect()
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

func (c *conn) reader() {
	for {
		// Get the current netclnt connection (which may change if the server
		// becomes unavailable and the writer thread connects to a new replica).
		c.Lock()
		if c.closed {
			return
		}
		nc := c.nc
		c.Unlock()

		// Receive the next reply.
		reply, err := nc.Recv()
		if err != nil {
			// If the connection broke, establish a new netclnt.
			c.Lock()
			// Check if the writer thread already reconnected to the replicas.
			if nc == c.nc {
				err := c.connect()
				if err != nil {
					// If no replicas are available, kill all remaining requests.
					log.Printf("Error %v SessionConn connecting to %v", err, c.addrs)
					c.close()
					return
				}
			}
			// Resend outstanding requests.
			c.resendOutstanding()
			c.Unlock()
			continue
		}
		rpc := c.outstanding[reply.Seqno]
		delete(c.outstanding, reply.Seqno)
		rpc.ReplyC <- &netclnt.Reply{reply, err}
	}
}

func (c *conn) writer() {
	c.Lock()
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
			db.DLPrintf("SESSIONCONN", "Error %v RPC to %v\n", err, c.nc.Dst())
			log.Printf("Error %v SessionConn couldn't RPC to %v", err, c.nc.Dst())
			err := c.connect()
			if err != nil {
				log.Printf("Error %v SessionConn connecting to %v", err, c.addrs)
				c.close()
				return
			}
			// Resend outstanding requests.
			c.resendOutstanding()
		}
	}
}

// Caller holds lock.
func (c *conn) resendOutstanding() {
	outstanding := make([]*netclnt.Rpc, len(c.outstanding))
	for i, o := range c.outstanding {
		outstanding[i] = o
	}
	sort.Slice(outstanding, func(i, j int) bool {
		return outstanding[i].Req.Seqno < outstanding[j].Req.Seqno
	})
	// Append outstanding requests that need to be resent to the front of the
	// queue.
	c.queue = append(outstanding, c.queue...)
	c.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
	// Signal that there are queued requests ready to be processed.
	c.Signal()
}

func (c *conn) close() {
	c.Lock()
	defer c.Unlock()
	c.nc.Close()
	c.closed = true
	// Kill outstanding requests.
	for _, o := range c.outstanding {
		close(o.ReplyC)
	}
}
