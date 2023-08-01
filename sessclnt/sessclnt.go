package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	"time"

	db "sigmaos/debug"
	"sigmaos/intervals"
	"sigmaos/netclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessconn"
	"sigmaos/sessp"
	"sigmaos/sessstateclnt"
	sp "sigmaos/sigmap"
)

// A session from a client to a logical server (either one server or a
// replica group)
type SessClnt struct {
	sync.Mutex
	*sync.Cond
	cli     sessp.Tclient
	sid     sessp.Tsession
	seqno   sessp.Tseqno
	closed  bool
	addrs   sp.Taddrs
	nc      *netclnt.NetClnt
	queue   *sessstateclnt.RequestQueue
	ivs     *intervals.Intervals
	clntnet string
}

func makeSessClnt(cli sessp.Tclient, clntnet string, addrs sp.Taddrs) (*SessClnt, *serr.Err) {
	c := &SessClnt{}
	c.cli = cli
	c.sid = sessp.Tsession(rand.Uint64())
	c.seqno = 0
	c.addrs = addrs
	c.Cond = sync.NewCond(&c.Mutex)
	c.nc = nil
	c.clntnet = clntnet
	c.queue = sessstateclnt.MakeRequestQueue(addrs)
	db.DPrintf(db.SESS_STATE_CLNT, "Cli %v make session %v to srvs %v", c.cli, c.sid, addrs)
	nc, err := netclnt.MakeNetClnt(c, clntnet, addrs)
	if err != nil {
		return nil, err
	}
	c.nc = nc
	c.ivs = intervals.MkIntervals(c.sid)
	go c.writer()
	return c, nil
}

func (c *SessClnt) RPC(req sessp.Tmsg, data []byte, f *sessp.Tfence) (*sessp.FcallMsg, *serr.Err) {
	rpc, err := c.send(req, data, f)
	if err != nil {
		db.DPrintf(db.SESS_STATE_CLNT, "%v Unable to send req %v %v err %v to %v\n", c.sid, req.Type(), req, err, c.addrs)
		return nil, err
	}
	rep, err1 := c.recv(rpc)
	if err1 != nil {
		db.DPrintf(db.SESS_STATE_CLNT, "%v Unable to recv response to req %v %v seqno %v err %v from %v\n", c.sid, req.Type(), rpc.Req.Fcm.Fc.Seqno, req, err1, c.addrs)
		return nil, err1
	}
	if db.WillBePrinted(db.SESS_STATE_CLNT) {
		db.DPrintf(db.SESS_STATE_CLNT, "%v RPC Successful, returning req %v %v seqno %v reply %v %v from %v\n", c.sid, req.Type(), rpc.Req.Fcm.Fc.Seqno, req, rep.Type(), rep, c.addrs)
	}
	return rep, err1
}

func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(sp.MkTheartbeat(map[uint64]bool{uint64(c.sid): true}), nil, sessp.NullFence())
	if err != nil {
		db.DPrintf(db.SESS_STATE_CLNT_ERR, "%v heartbeat %v err %v", c.sid, c.addrs, err)
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
	db.DPrintf(db.SESS_STATE_CLNT, "%v Reset outstanding request queue to %v", c.sid, c.addrs)
	c.queue.Reset()
	// Reset intervals "next" slice so we can resend message acks.
	c.ivs.ResetNext()
	// Try to send a heartbeat to force a reconnect to the replica group.
	go c.sendHeartbeat()
}

// Complete an RPC and pass the response up the stack.
func (c *SessClnt) CompleteRPC(seqno sessp.Tseqno, f []byte, d []byte, err *serr.Err) {
	rpc, ok := c.queue.Remove(seqno)
	// the outstanding request may have been cleared if the conn is closing, or
	// if a previous version of this request was sent and received, in which case
	// rpc == nil and ok == false.
	if ok {
		db.DPrintf(db.SESS_STATE_CLNT, "%v Complete rpc req %v from %v", c.sid, rpc.Req, c.addrs)
		rpc.Complete(f, d, err)
	} else {
		db.DPrintf(db.SESS_STATE_CLNT, "%v Already completed rpc from %v; seqnos %v\n", c.sid, c.addrs, c.ivs)
	}
}

// Send a detach.
func (c *SessClnt) Detach(cid sp.TclntId) *serr.Err {
	db.DPrintf(db.ALWAYS, "%v: Send detach %v\n", proc.GetPid(), c.sid)
	rep, err := c.RPC(sp.MkTdetach(0, 0, cid), nil, sessp.NullFence())
	if err != nil {
		db.DPrintf(db.SESS_STATE_CLNT_ERR, "detach %v err %v", c.sid, err)
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
func srvClosedSess(msg sessp.Tmsg, err *serr.Err) bool {
	if msg.Type() == sessp.TRdetach {
		return true
	}
	if rerr, ok := msg.(*sp.Rerror); ok {
		err := sp.MkErr(rerr)
		if err.IsErrSessClosed() {
			return true
		}
	}
	return false
}

func (c *SessClnt) send(req sessp.Tmsg, data []byte, f *sessp.Tfence) (*netclnt.Rpc, *serr.Err) {
	s := time.Now()

	// If the request is not an RPC, we need to ensure strict ordering by seqno.
	// So, hold the lock between incrementing the seqno (which happens in
	// sessp.MakeFcallMsg) and enqueueing the message. This holds the lock during
	// marshaling.
	if req.Type() != sessp.TTwriteread {
		c.Lock()
		defer c.Unlock()
	}

	// For TTwriteread (RPCs) we make no ordering guarantees. This allows us to
	// avoid holding the lock between the Fcall message creation step (which
	// allocates a sequence number), the marshaling step (which often takes a
	// long time), and the request enqueue step (which ordinarily expects fcalls
	// to be enqueued in order).
	fc := sessp.MakeFcallMsg(req, data, c.cli, c.sid, &c.seqno, c.ivs.Next(), f)
	rpc := netclnt.MakeRpc(c.addrs, sessconn.MakePartMarshaledMsg(fc), s)

	// If the request is an RPC, then we don't have strict ordering requirements.
	// We haven't taken the lock yet in order to avoid holding the lock while
	// marshaling the message, which may take a long time. However, we need to
	// take the lock here to ensure the status of the session (c.closed) is
	// checked atomically with the RPC enqueueing.
	if req.Type() == sessp.TTwriteread {
		c.Lock()
		defer c.Unlock()
	}

	if c.closed {
		return nil, serr.MkErr(serr.TErrUnreachable, c.addrs)
	}

	// Enqueue a request
	c.queue.Enqueue(rpc)
	return rpc, nil
}

// Wait for an RPC to be completed. When this happens, we reset the heartbeat
// timer.
func (c *SessClnt) recv(rpc *netclnt.Rpc) (*sessp.FcallMsg, *serr.Err) {
	reply, err := rpc.Await()
	// Reply may be nil if the server became unreachable, the session was closed,
	// and outstanding RPCs were aborted.
	if reply != nil {
		o := uint64(reply.Seqno())
		c.ivs.Insert(sessp.MkInterval(o, o+1))
		c.ivs.Delete(reply.Fc.Received)
		db.DPrintf(db.SESS_STATE_CLNT, "%v Complete rpc req %v reply %v from %v; seqnos %v\n", c.sid, rpc.Req, reply, c.addrs, c.ivs)
		// If the server closed the session (this is a sessclosed error or an
		// Rdetach), close the SessClnt.
		if srvClosedSess(reply.Msg, err) {
			db.DPrintf(db.SESS_STATE_CLNT, "Srv %v closed sess %v on req seqno %v\n", c.addrs, c.sid, reply.Seqno())
			c.close()
		}
	}
	return reply, err
}

// Get a connection to the server. If it isn't possible to make a connection,
// return an error.
func (c *SessClnt) getConn() (*netclnt.NetClnt, *serr.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, serr.MkErr(serr.TErrUnreachable, c.addrs)
	}

	if c.nc == nil {
		db.DPrintf(db.SESS_STATE_CLNT, "%v SessionConn reconnecting to %v %v\n", c.sid, c.addrs, c.closed)
		nc, err := netclnt.MakeNetClnt(c, c.clntnet, c.addrs)
		if err != nil {
			db.DPrintf(db.SESS_STATE_CLNT, "%v Error %v unable to reconnect to %v\n", c.sid, err, c.addrs)
			return nil, err
		}
		db.DPrintf(db.SESS_STATE_CLNT, "%v Successful connection to %v out of %v\n", c.sid, nc.Dst(), c.addrs)
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
	db.DPrintf(db.SESS_STATE_CLNT, "%v Close session to %v %v\n", c.sid, c.addrs, c.closed)
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

		s := time.Now()
		nc, err := c.getConn()
		if time.Since(s) > 100*time.Microsecond {
			db.DPrintf(db.SESS_LAT, "Long getconn %v", time.Since(s))
		}

		// If we can't connect to the replica group, return.
		if err != nil {
			c.close()
			break
		}

		nc.Send(req)
	}
	db.DPrintf(db.SESS_STATE_CLNT, "%v writer returns %v %v\n", c.sid, c.addrs, c.closed)
}
