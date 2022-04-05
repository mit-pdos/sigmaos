package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
	"ulambda/sessstateclnt"
)

// A session from a client to a logical server (either one server or a
// replica group)
type SessClnt struct {
	sync.Mutex
	*sync.Cond
	sid         np.Tsession
	seqno       *np.Tseqno
	closed      bool
	addrs       []string
	nc          *netclnt.NetClnt
	queue       *sessstateclnt.RequestQueue
	lastMsgTime time.Time
}

func makeSessClnt(sid np.Tsession, seqno *np.Tseqno, addrs []string) (*SessClnt, *np.Err) {
	c := &SessClnt{}
	c.sid = sid
	c.seqno = seqno
	c.addrs = addrs
	c.Cond = sync.NewCond(&c.Mutex)
	c.nc = nil
	c.queue = sessstateclnt.MakeRequestQueue()
	err := c.connect()
	if err != nil {
		return nil, err
	}
	go c.reader()
	go c.writer()
	go c.heartbeats()
	return c, nil
}

func (c *SessClnt) rpc(req np.Tmsg, f np.Tfence) (np.Tmsg, *np.Err) {
	rpc, err := c.send(req, f)
	if err != nil {
		db.DPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", c.sid, req.Type(), req, err, c.addrs)
		return nil, err
	}
	rep, err1 := c.recv(rpc)
	if err1 != nil {
		db.DPrintf("SESSCLNT", "%v Unable to recv response to req %v %v err %v from %v\n", c.sid, req.Type(), req, err1, c.addrs)
		return nil, err1
	}
	return rep, err1
}

func (c *SessClnt) send(req np.Tmsg, f np.Tfence) (*netclnt.Rpc, *np.Err) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, np.MkErr(np.TErrUnreachable, c.addrs)
	}

	rpc := netclnt.MakeRpc(np.MakeFcall(req, c.sid, c.seqno, f))
	// Enqueue a request
	c.queue.Enqueue(rpc)
	return rpc, nil
}

func (c *SessClnt) recv(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
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

func (c *SessClnt) connect() *np.Err {
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
func (c *SessClnt) tryReconnect(oldNc *netclnt.NetClnt) *np.Err {
	c.Lock()
	defer c.Unlock()
	// Check if another thread already reconnected to the replicas.
	if oldNc == c.nc {
		return c.tryReconnectL()
	}
	return nil
}

// Reconnect & resend requests
func (c *SessClnt) tryReconnectL() *np.Err {
	if c.closed {
		return np.MkErr(np.TErrUnreachable, "c closed")
	}
	db.DPrintf("SESSCLNT", "%v SessionConn reconnecting to %v %v\n", c.sid, c.addrs, c.closed)
	err := c.connect()
	if err != nil {
		db.DPrintf("SESSCLNT", "%v Error %v SessionConn reconnecting to %v\n", c.sid, err, c.addrs)
		return err
	}
	// Resend outstanding requests.
	db.DPrintf("SESSCLNT", "%v Resend outstanding requests to %v\n", c.sid, c.addrs)
	c.queue.Reset()
	return nil
}

// Complete an RPC and send a response.
func (c *SessClnt) completeRpc(reply *np.Fcall, err *np.Err) {
	rpc, ok := c.queue.Complete(reply.Seqno)
	// the outstanding request may have been cleared if the conn is closing,
	// in which case rpc will be nil.
	if ok {
		db.DPrintf("SESSCLNT", "%v Complete rpc req %v reply %v from %v\n", c.sid, rpc.Req, reply, c.addrs)
		rpc.ReplyC <- &netclnt.Reply{reply, err}
		close(rpc.ReplyC)
	}
}

// Caller holds lock.
func (c *SessClnt) resendOutstanding() {
}

func (c *SessClnt) isClosed() bool {
	c.Lock()
	defer c.Unlock()
	return c.closed
}

func (c *SessClnt) getNc() *netclnt.NetClnt {
	c.Lock()
	defer c.Unlock()
	return c.nc
}

func (c *SessClnt) sessClose() {
	c.Lock()
	defer c.Unlock()
	c.close()
	// delete(sc.sessions, addr)
}

func (c *SessClnt) SessDetach() *np.Err {
	rep, err := c.rpc(np.Tdetach{0, 0}, np.NoFence)
	if err != nil {
		db.DPrintf("SESSCLNT_ERR", "detach %v err %v", c.sid, err)
	}
	rmsg, ok := rep.(np.Rerror)
	if ok {
		err := np.String2Err(rmsg.Ename)
		return err
	}
	return nil
}

// Caller holds lock
func (c *SessClnt) close() {
	if c.closed {
		return
	}
	c.closed = true
	db.DPrintf("SESSCLNT", "%v Close session to %v %v\n", c.sid, c.addrs, c.closed)
	c.nc.Close()
	outstanding := c.queue.Close()
	// Kill outstanding requests.
	for _, o := range outstanding {
		close(o.ReplyC)
	}
}

func (c *SessClnt) needsHeartbeat() bool {
	c.Lock()
	defer c.Unlock()
	return !c.closed && time.Now().Sub(c.lastMsgTime) >= np.SESSHEARTBEATMS
}

func (c *SessClnt) heartbeats() {
	for !c.isClosed() {
		// Sleep a bit.
		time.Sleep(np.SESSHEARTBEATMS * time.Millisecond)
		if c.needsHeartbeat() {
			// XXX How soon should I retry if this fails?
			db.DPrintf("SESSCLNT", "%v Sending heartbeat to %v", c.sid, c.addrs)
			rep, err := c.rpc(np.Theartbeat{[]np.Tsession{c.sid}}, np.NoFence)
			if err != nil {
				db.DPrintf("SESSCLNT_ERR", "%v heartbeat %v err %v", c.sid, c.addrs, err)
				continue
			}
			if c.srvClosedSess(rep) {
				c.sessClose()
				return
			}
		}
	}
}

func (c *SessClnt) srvClosedSess(reply np.Tmsg) bool {
	if reply.Type() == np.TRerror {
		rmsg := reply.(np.Rerror)
		err := np.String2Err(rmsg.Ename)
		db.DPrintf("SESSCLNT_ERR", "%v sessclose %v reply %v", c.sid, c.addrs, err)
		if np.IsErrClosed(err) {
			return true
		}
	}
	return false
}

func (c *SessClnt) reader() {
	for !c.isClosed() {
		// Get the current netclnt connection (which may
		// change if the server becomes unavailable)
		nc := c.getNc()

		// Receive the next reply.
		reply, err := nc.Recv()
		if err != nil {
			db.DPrintf("SESSCLNT", "%v error %v reader RPC to %v", c.sid, err, c.addrs)
			err := c.tryReconnect(nc)
			if err != nil {
				// If we can't reconnect, close the session.
				c.sessClose()
				return
			}
			// If the connection broke, establish a new netclnt.
			continue
		}
		c.completeRpc(reply, err)

		if reply.Msg.Type() == np.TRdetach || c.srvClosedSess(reply.Msg) {
			// close session now; so that the reader
			// doesn't retry because the other side may
			// have already closed TCP connection.
			c.sessClose()
		}
	}
}

func (c *SessClnt) writer() {
	for {
		// Try to get the next request to be sent
		req := c.queue.Next()
		if req == nil {
			return
		}
		nc := c.getNc()

		err := nc.Send(req)
		if err != nil {
			db.DPrintf("SESSCLNT_ERR", "%v Error %v writer RPC to %v\n", c.sid, err, nc.Dst())
			err := c.tryReconnect(nc)
			if err != nil {
				db.DPrintf("SESSCLNT_ERR", "Writer: c close() %v", c.sid)
				c.sessClose()
				return
			}
		}
	}
}
