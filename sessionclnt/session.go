package sessionclnt

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
type sessclnt struct {
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

func makeConn(sid np.Tsession, seqno *np.Tseqno, addrs []string) (*sessclnt, *np.Err) {
	sess := &sessclnt{}
	sess.sid = sid
	sess.seqno = seqno
	sess.addrs = addrs
	sess.Cond = sync.NewCond(&sess.Mutex)
	sess.nc = nil
	sess.queue = []*netclnt.Rpc{}
	sess.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
	err := sess.connect()
	if err != nil {
		return nil, err
	}
	go sess.reader()
	go sess.writer()
	go sess.heartbeats()
	return sess, nil
}

func (sess *sessclnt) rpc(req np.Tmsg, f np.Tfence) (np.Tmsg, *np.Err) {
	rpc, err := sess.send(req, f)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", sess.sid, req.Type(), req, err, sess.addrs)
		return nil, err
	}
	rep, err1 := sess.recv(rpc)
	if err1 != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to recv response to req %v %v err %v from %v\n", sess.sid, req.Type(), req, err, sess.addrs)
		return nil, err1
	}
	return rep, err1
}

func (sess *sessclnt) send(req np.Tmsg, f np.Tfence) (*netclnt.Rpc, *np.Err) {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		return nil, np.MkErr(np.TErrUnreachable, sess.addrs)
	}

	rpc := netclnt.MakeRpc(np.MakeFcall(req, sess.sid, sess.seqno, f))
	// Enqueue a request
	sess.queue = append(sess.queue, rpc)
	sess.outstanding[rpc.Req.Seqno] = rpc
	sess.Signal()
	return rpc, nil
}

func (sess *sessclnt) recv(rpc *netclnt.Rpc) (np.Tmsg, *np.Err) {
	// Wait for a reply
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, np.MkErr(np.TErrUnreachable, sess.addrs)
	}
	sess.Lock()
	defer sess.Unlock()
	sess.lastMsgTime = time.Now()
	return reply.Fc.Msg, reply.Err
}

func (sess *sessclnt) connect() *np.Err {
	db.DLPrintf("SESSCLNT", "%v Connect to %v\n", sess.sid, sess.addrs)
	for _, addr := range sess.addrs {
		nc, err := netclnt.MakeNetClnt(addr)
		// If this replica is unreachable, try another one.
		if err != nil {
			continue
		}
		db.DLPrintf("SESSCLNT", "%v Successful connection to %v out of %v\n", sess.sid, addr, sess.addrs)
		// If the replica is reachable, save this conn.
		sess.nc = nc
		return nil
	}
	db.DLPrintf("SESSCLNT", "%v Unable to connect to %v\n", sess.sid, sess.addrs)
	// No replica is reachable.
	return np.MkErr(np.TErrUnreachable, sess.addrs)
}

// If the connection broke, establish a new netclnt connection. If successful,
// resend outstanding requests.
func (sess *sessclnt) tryReconnect(oldNc *netclnt.NetClnt) *np.Err {
	sess.Lock()
	defer sess.Unlock()
	// Check if another thread already reconnected to the replicas.
	if oldNc == sess.nc {
		return sess.tryReconnectL()
	}
	return nil
}

// Reconnect & resend requests
func (sess *sessclnt) tryReconnectL() *np.Err {
	err := sess.connect()
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Error %v SessionConn reconnecting to %v\n", sess.sid, err, sess.addrs)
		return err
	}
	// Resend outstanding requests.
	sess.resendOutstanding()
	return nil
}

// Complete an RPC and send a response.
func (sess *sessclnt) completeRpc(reply *np.Fcall, err *np.Err) {
	sess.Lock()
	rpc, ok := sess.outstanding[reply.Seqno]
	delete(sess.outstanding, reply.Seqno)
	sess.Unlock()
	// the outstanding request map may have been cleared if the conn is closing,
	// in which case rpc will be nil.
	if ok && !rpc.Done {
		db.DLPrintf("SESSCLNT", "%v Complete rpc req %v reply %v from %v\n", sess.sid, rpc.Req, reply, sess.addrs)
		rpc.Done = true
		rpc.ReplyC <- &netclnt.Reply{reply, err}
	}
}

// Caller holds lock.
func (sess *sessclnt) resendOutstanding() {
	db.DLPrintf("SESSCLNT", "%v Resend outstanding requests to %v\n", sess.sid, sess.addrs)
	outstanding := make([]*netclnt.Rpc, len(sess.outstanding))
	idx := 0
	for _, o := range sess.outstanding {
		outstanding[idx] = o
		idx++
	}
	sort.Slice(outstanding, func(i, j int) bool {
		return outstanding[i].Req.Seqno < outstanding[j].Req.Seqno
	})
	// Append outstanding requests that need to be resent to the front of the
	// queue.
	sess.queue = append(outstanding, sess.queue...)
	// Signal that there are queued requests ready to be processed.
	sess.Signal()
}

func (sess *sessclnt) done() bool {
	sess.Lock()
	defer sess.Unlock()
	return sess.closed
}

// Caller holds lock
func (sess *sessclnt) close() {
	db.DLPrintf("SESSCLNT", "%v Close conn to %v\n", sess.sid, sess.addrs)
	sess.nc.Close()
	sess.closed = true
	// Kill pending requests.
	for _, o := range sess.queue {
		if !o.Done {
			o.Done = true
			close(o.ReplyC)
		}
	}
	// Kill outstanding requests.
	for _, o := range sess.outstanding {
		if !o.Done {
			o.Done = true
			close(o.ReplyC)
		}
	}
	sess.queue = []*netclnt.Rpc{}
	sess.outstanding = make(map[np.Tseqno]*netclnt.Rpc)
}

func (sess *sessclnt) needsHeartbeat() bool {
	sess.Lock()
	defer sess.Unlock()
	return time.Now().Sub(sess.lastMsgTime) >= np.SESSHEARTBEATMS
}

func (sess *sessclnt) heartbeats() {
	for !sess.done() {
		// Sleep a bit.
		time.Sleep(np.SESSHEARTBEATMS * time.Millisecond)
		if sess.needsHeartbeat() {
			// XXX How soon should I retry if this fails?
			db.DLPrintf("SESSCLNT", "%v Sending heartbeat to %v", sess.sid, sess.addrs)
			sess.rpc(np.Theartbeat{[]np.Tsession{sess.sid}}, np.NoFence)
		}
	}
}

func (sess *sessclnt) reader() {
	for !sess.done() {
		// Get the current netclnt connection (which may change if the server
		// becomes unavailable and the writer thread connects to a new replica).
		sess.Lock()
		nc := sess.nc
		sess.Unlock()

		// Receive the next reply.
		reply, err := nc.Recv()
		if err != nil {
			db.DLPrintf("SESSCLNT", "%v error %v reader RPC to %v", sess.sid, err, sess.addrs)
			// Try to connect to the next replica
			err := sess.tryReconnect(nc)
			if err != nil {
				// If we can't reconnect to any of the replicas, close the session.
				sess.Lock()
				sess.close()
				sess.Unlock()
				return
			}
			// If the connection broke, establish a new netclnt.
			continue
		}
		sess.completeRpc(reply, err)
	}
}

func (sess *sessclnt) writer() {
	sess.Lock()
	defer sess.Unlock()
	for {
		var req *netclnt.Rpc
		// Wait until we have an RPC request.
		for len(sess.queue) == 0 {
			if sess.closed {
				return
			}
			sess.Wait()
		}
		// Pop the first item form the queue.
		req, sess.queue = sess.queue[0], sess.queue[1:]
		err := sess.nc.Send(req)
		if err != nil {
			db.DLPrintf("SESSCLNT", "%v Error %v writer RPC to %v\n", sess.sid, err, sess.nc.Dst())
			err := sess.tryReconnectL()
			if err != nil {
				sess.close()
				return
			}
		}
	}
}
