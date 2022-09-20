package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	db "sigmaos/debug"
	"sigmaos/intervals"
	"sigmaos/netclnt"
	np "sigmaos/ninep"
	"sigmaos/rand"
	"sigmaos/sessstateclnt"
)

// A session from a client to a logical server (either one server or a
// replica group)
type SessClnt struct {
	sync.Mutex
	*sync.Cond
	cli    np.Tclient
	sid    np.Tsession
	seqno  np.Tseqno
	closed bool
	addrs  []string
	nc     *netclnt.NetClnt
	queue  *sessstateclnt.RequestQueue
	ivs    *intervals.Intervals
}

func makeSessClnt(cli np.Tclient, addrs []string) (*SessClnt, *np.Err) {
	c := &SessClnt{}
	c.cli = cli
	c.sid = np.Tsession(rand.Uint64())
	c.seqno = 0
	c.addrs = addrs
	c.Cond = sync.NewCond(&c.Mutex)
	c.nc = nil
	c.queue = sessstateclnt.MakeRequestQueue(addrs)
	db.DPrintf("SESSCLNT", "Cli %v make session %v to srvs %v", c.cli, c.sid, addrs)
	nc, err := netclnt.MakeNetClnt(c, addrs)
	if err != nil {
		return nil, err
	}
	c.nc = nc
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
	db.DPrintf("SESSCLNT", "%v RPC Successful, returning req %v %v seqno %v reply %v %v from %v\n", c.sid, req.Type(), rpc.Req.Seqno, req, rep.Type(), rep, c.addrs)
	return rep, err1
}

func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(np.Theartbeat{[]np.Tsession{c.sid}}, np.NoFence)
	if err != nil {
		db.DPrintf("SESSCLNT_ERR", "%v heartbeat %v err %v", c.sid, c.addrs, err)
	}
}

// Clear the connection, reset the request queue, and enqueue a heartbeat to
// re-establish a connection with the replica group if possible.
func (c *SessClnt) Reset() {
	c.Lock()
	defer c.Unlock()

	if c.nc != nil {
		c.nc = nil
	}
	// Reset outstanding request queue.
	db.DPrintf("SESSCLNT", "%v Reset outstanding request queue to %v", c.sid, c.addrs)
	c.queue.Reset()
	// Try to send a heartbeat to force a reconnect to the replica group.
	go c.sendHeartbeat()
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
		c.ivs.Delete(&reply.Received)
		db.DPrintf("SESSCLNT", "%v Complete rpc req %v reply %v from %v; seqnos %v\n", c.sid, rpc.Req, reply, c.addrs, c.ivs)
		rpc.Complete(reply, err)
	} else {
		db.DPrintf("SESSCLNT", "%v Already completed rpc reply %v from %v; seqnos %v\n", c.sid, reply, c.addrs, c.ivs)
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
	rep, err := c.RPC(np.Tdetach{0, 0}, np.NoFence)
	if err != nil {
		db.DPrintf("SESSCLNT_ERR", "detach %v err %v", c.sid, err)
	}
	rmsg, ok := rep.(*np.Rerror)
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
	rerr, ok := msg.(*np.Rerror)
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
	rpc := netclnt.MakeRpc(c.addrs, np.MakeFcall(req, c.cli, c.sid, &c.seqno, c.ivs.First(), f))
	// Enqueue a request
	c.queue.Enqueue(rpc)
	return rpc, nil
}

// Wait for an RPC to be completed. When this happens, we reset the heartbeat
// timer.
func (c *SessClnt) recv(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
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
