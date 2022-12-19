package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/intervals"
	"sigmaos/netclnt"
	"sigmaos/rand"
	"sigmaos/sessstateclnt"
	sp "sigmaos/sigmap"
)

// A session from a client to a logical server (either one server or a
// replica group)
type SessClnt struct {
	sync.Mutex
	*sync.Cond
	cli    fcall.Tclient
	sid    fcall.Tsession
	seqno  sp.Tseqno
	closed bool
	addrs  []string
	nc     *netclnt.NetClnt
	queue  *sessstateclnt.RequestQueue
	ivs    *intervals.Intervals
}

func makeSessClnt(cli fcall.Tclient, addrs []string) (*SessClnt, *fcall.Err) {
	c := &SessClnt{}
	c.cli = cli
	c.sid = fcall.Tsession(rand.Uint64())
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

func (c *SessClnt) RPC(req fcall.Tmsg, data []byte, f *sp.Tfence) (*sp.FcallMsg, *fcall.Err) {
	rpc, err := c.send(req, data, f)
	if err != nil {
		db.DPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", c.sid, req.Type(), req, err, c.addrs)
		return nil, err
	}
	rep, err1 := c.recv(rpc)
	if err1 != nil {
		db.DPrintf("SESSCLNT", "%v Unable to recv response to req %v %v seqno %v err %v from %v\n", c.sid, req.Type(), rpc.Req.Fc.Seqno, req, err1, c.addrs)
		return nil, err1
	}
	if db.WillBePrinted("SESSCLNT") {
		db.DPrintf("SESSCLNT", "%v RPC Successful, returning req %v %v seqno %v reply %v %v from %v\n", c.sid, req.Type(), rpc.Req.Fc.Seqno, req, rep.Type(), rep, c.addrs)
	}
	return rep, err1
}

func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(sp.MkTheartbeat([]uint64{uint64(c.sid)}), nil, sp.MakeFenceNull())
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
func (c *SessClnt) CompleteRPC(reply *sp.FcallMsg, err *fcall.Err) {
	s := reply.Seqno()
	rpc, ok := c.queue.Remove(s)
	// the outstanding request may have been cleared if the conn is closing, or
	// if a previous version of this request was sent and received, in which case
	// rpc == nil and ok == false.
	if ok {
		o := reply.Fc.Seqno
		c.ivs.Insert(sp.MkInterval(o, o+1))
		c.ivs.Delete(reply.Fc.Received)
		db.DPrintf("SESSCLNT", "%v Complete rpc req %v reply %v from %v; seqnos %v\n", c.sid, rpc.Req, reply, c.addrs, c.ivs)
		rpc.Complete(reply, err)
	} else {
		db.DPrintf("SESSCLNT", "%v Already completed rpc reply %v from %v; seqnos %v\n", c.sid, reply, c.addrs, c.ivs)
	}
	// If the server closed the session (this is a sessclosed error or an
	// Rdetach), close the SessClnt.
	if srvClosedSess(reply.Msg, err) {
		db.DPrintf("SESSCLNT", "Srv %v closed sess %v on req seqno %v\n", c.addrs, c.sid, s)
		c.close()
	}
}

// Send a detach.
func (c *SessClnt) Detach() *fcall.Err {
	rep, err := c.RPC(sp.MkTdetach(0, 0), nil, sp.MakeFenceNull())
	if err != nil {
		db.DPrintf("SESSCLNT_ERR", "detach %v err %v", c.sid, err)
		return err
	}
	rmsg, ok := rep.Msg.(*sp.Rerror)
	if ok {
		return sp.MkErr(rmsg)
	}
	return nil
}

// Check if the session needs to be closed, either because the server killed
// it, or because the client called detach. Close will be called in CompleteRPC
// once the Rdetach is received.
func srvClosedSess(msg fcall.Tmsg, err *fcall.Err) bool {
	if msg.Type() == fcall.TRdetach {
		return true
	}
	rerr, ok := msg.(*sp.Rerror)
	if ok {
		err := sp.MkErr(rerr)
		if fcall.IsErrSessClosed(err) {
			return true
		}
	}
	return false
}

func (c *SessClnt) send(req fcall.Tmsg, data []byte, f *sp.Tfence) (*netclnt.Rpc, *fcall.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, fcall.MkErr(fcall.TErrUnreachable, c.addrs)
	}
	rpc := netclnt.MakeRpc(c.addrs, sp.MakeFcallMsg(req, data, c.cli, c.sid, &c.seqno, c.ivs.First(), f))
	// Enqueue a request
	c.queue.Enqueue(rpc)
	return rpc, nil
}

// Wait for an RPC to be completed. When this happens, we reset the heartbeat
// timer.
func (c *SessClnt) recv(rpc *netclnt.Rpc) (*sp.FcallMsg, *fcall.Err) {
	return rpc.Await()
}

// Get a connection to the server. If it isn't possible to make a connection,
// return an error.
func (c *SessClnt) getConn() (*netclnt.NetClnt, *fcall.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, fcall.MkErr(fcall.TErrUnreachable, c.addrs)
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
