package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	db "ulambda/debug"
	"ulambda/intervals"
	"ulambda/netclnt"
	np "ulambda/ninep"
	"ulambda/sessstateclnt"
)

// A session from a client to a logical server (either one server or a
// replica group)
type SessClnt struct {
	sync.Mutex
	*sync.Cond
	sid    np.Tsession
	seqno  *np.Tseqno
	closed bool
	addrs  []string
	nc     *netclnt.NetClnt
	queue  *sessstateclnt.RequestQueue
	hb     *Heartbeater
	ivs    *intervals.Intervals
}

func makeSessClnt(sid np.Tsession, seqno *np.Tseqno, addrs []string) (*SessClnt, *np.Err) {
	c := &SessClnt{}
	c.sid = sid
	c.seqno = seqno
	c.addrs = addrs
	c.Cond = sync.NewCond(&c.Mutex)
	c.nc = nil
	c.queue = sessstateclnt.MakeRequestQueue()
	nc, err := netclnt.MakeNetClnt(c, addrs)
	if err != nil {
		return nil, err
	}
	c.nc = nc
	c.hb = makeHeartbeater(c)
	c.ivs = intervals.MkIntervals()
	go c.writer()
	return c, nil
}

func (c *SessClnt) RPC(req np.Tmsg, f np.Tfence) (np.Tmsg, *np.Err) {
	rpc, err := c.send(req, f)
	if err != nil {
		db.DPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", c.sid, req.Type(), req, err, c.addrs)
		return nil, err
	}
	rep, err1 := c.recv(rpc)
	if err1 != nil {
		db.DPrintf("SESSCLNT", "%v Unable to recv response to req %v %v seqno %v err %v from %v\n", c.sid, req.Type(), rpc.Req.Seqno, req, err1, c.addrs)
		return nil, err1
	}
	return rep, err1
}

// Clear the connection and reset the request queue.
func (c *SessClnt) Reset() {
	c.Lock()
	defer c.Unlock()

	if c.nc != nil {
		c.nc = nil
	}
	// Reset outstanding request queue.
	db.DPrintf("SESSCLNT", "%v Reset outstanding request queue to %v\n", c.sid, c.addrs)
	c.queue.Reset()
}

// Complete an RPC and pass the response up the stack.
func (c *SessClnt) CompleteRPC(reply *np.Fcall, err *np.Err) {
	rpc, ok := c.queue.Remove(reply.Seqno)
	// the outstanding request may have been cleared if the conn is closing, or
	// if a previous version of this request was sent and received, in which case
	// rpc == nil and ok == false.
	if ok {
		o := np.Toffset(reply.Seqno)
		c.ivs.Insert(np.MkInterval(o, o+1))
		db.DPrintf("SESSCLNT", "%v Complete rpc req %v reply %v from %v; seqnos %v\n", c.sid, rpc.Req, reply, c.addrs, c.ivs)
		rpc.Complete(reply, err)
	}
	// If the server closed the session (this is a sessclosed error or an
	// Rdetach), close the SessClnt.
	if srvClosedSess(reply.Msg, err) {
		db.DPrintf("SESSCLNT", "Srv %v closed sess %v on req seqno %v\n", c.addrs, c.sid, reply.Seqno)
		c.close()
	}
}

// Send a detach.
func (c *SessClnt) Detach() *np.Err {
	// Stop the heartbeater.
	c.hb.Stop()
	rep, err := c.RPC(np.Tdetach{0, 0}, np.NoFence)
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

// Check if the session needs to be closed, either because the server killed
// it, or because the client called detach. Close will be called in CompleteRPC
// once the Rdetach is received.
func srvClosedSess(msg np.Tmsg, err *np.Err) bool {
	if msg.Type() == np.TRdetach {
		return true
	}
	rerr, ok := msg.(np.Rerror)
	if ok {
		err := np.String2Err(rerr.Ename)
		if np.IsErrSessClosed(err) {
			return true
		}
	}
	return false
}

func (c *SessClnt) send(req np.Tmsg, f np.Tfence) (*netclnt.Rpc, *np.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, np.MkErr(np.TErrUnreachable, c.addrs)
	}

	rpc := netclnt.MakeRpc(c.addrs, np.MakeFcall(req, c.sid, c.seqno, f))
	// Enqueue a request
	c.queue.Enqueue(rpc)
	return rpc, nil
}

// Wait for an RPC to be completed. When this happens, we reset the heartbeat
// timer.
func (c *SessClnt) recv(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
	defer c.hb.HeartbeatAckd()
	return rpc.Await()
}

// Get a connection to the server. If it isn't possible to make a connection,
// return an error.
func (c *SessClnt) getConn() (*netclnt.NetClnt, *np.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, np.MkErr(np.TErrUnreachable, c.addrs)
	}

	if c.nc == nil {
		db.DPrintf("SESSCLNT", "%v SessionConn reconnecting to %v %v\n", c.sid, c.addrs, c.closed)
		nc, err := netclnt.MakeNetClnt(c, c.addrs)
		if err != nil {
			db.DPrintf("SESSCLNT", "%v Error %v unable to reconnect to %v\n", c.sid, err, c.addrs)
			return nil, err
		}
		db.DPrintf("SESSCLNT", "%v Successful connection to %v out of %v\n", c.sid, nc.Dst(), c.addrs)
		c.nc = nc
	}
	return c.nc, nil
}

func (c *SessClnt) isClosed() bool {
	c.Lock()
	defer c.Unlock()
	return c.closed
}

// Close the sessclnt connection to this replica group.
func (c *SessClnt) close() {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return
	}
	c.closed = true
	db.DPrintf("SESSCLNT", "%v Close session to %v %v\n", c.sid, c.addrs, c.closed)
	if c.nc != nil {
		c.nc.Close()
	}
	outstanding := c.queue.Close()
	// Kill outstanding requests.
	for _, rpc := range outstanding {
		rpc.Abort()
	}
}

func (c *SessClnt) writer() {
	for !c.isClosed() {
		// Try to get the next request to be sent
		req := c.queue.Next()

		if req == nil {
			break
		}

		nc, err := c.getConn()

		// If we can't connect to the replica group, return.
		if err != nil {
			c.close()
			break
		}

		nc.Send(req)
	}
	db.DPrintf("SESSCLNT", "%v writer returns %v %v\n", c.sid, c.addrs, c.closed)
}
