// The sessclnt package establishes a session with a server using a
// [netclnt], which sets up a TCP connection.  If [netclnt] fails,
// sessclnt may try to re-establish the connection.
package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	//"time"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netclnt"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type SessClnt struct {
	sync.Mutex
	sid     sessp.Tsession
	seqno   sessp.Tseqno
	closed  bool
	addrs   sp.Taddrs
	nc      *netclnt.NetClnt
	queue   *RequestQueue
	clntnet string
}

func newSessClnt(clntnet string, addrs sp.Taddrs, rf demux.ReadCallF, wf demux.WriteCallF) (*SessClnt, *serr.Err) {
	c := &SessClnt{sid: sessp.Tsession(rand.Uint64()),
		clntnet: clntnet,
		addrs:   addrs,
		queue:   NewRequestQueue(addrs),
	}
	db.DPrintf(db.SESSCLNT, "Make session %v to srvs %v", c.sid, addrs)
	nc, err := netclnt.NewNetClnt(clntnet, addrs, spcodec.ReadCall, spcodec.WriteCall, c)
	if err != nil {
		return nil, err
	}
	c.nc = nc
	return c, nil
}

func (c *SessClnt) SessId() sessp.Tsession {
	return c.sid
}

func (c *SessClnt) netClnt() *netclnt.NetClnt {
	c.Lock()
	defer c.Unlock()
	return c.nc
}

func (c *SessClnt) ownNetClnt() *netclnt.NetClnt {
	c.Lock()
	defer c.Unlock()
	r := c.nc
	c.nc = nil
	return r
}

func (c *SessClnt) IsConnected() bool {
	if nc := c.netClnt(); nc != nil {
		return !nc.IsClosed()
	}
	return false
}

// XXX if unreachable, nothing to be done (netconn is closed), but if
// marshaling error, close conn?
func (c *SessClnt) ReportError(err error) {
	db.DPrintf(db.SESSCLNT, "Netclnt sess %v reports err %v\n", c.sid, err)
}

func (c *SessClnt) RPC(req sessp.Tmsg, data []byte) (*sessp.FcallMsg, *serr.Err) {
	fc := sessp.NewFcallMsg(req, data, c.sid, &c.seqno)
	nc := c.netClnt()
	if nc == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, c.addrs)
	}

	rep, err := nc.SendReceive(fc)
	if err != nil {
		return nil, err
	}
	return rep.(*sessp.FcallMsg), nil
}

func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(sp.NewTheartbeat(map[uint64]bool{uint64(c.sid): true}), nil)
	if err != nil {
		db.DPrintf(db.SESSCLNT_ERR, "%v heartbeat %v err %v", c.sid, c.addrs, err)
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
	db.DPrintf(db.SESSCLNT, "%v Reset outstanding request queue to %v", c.sid, c.addrs)
	c.queue.Reset()
	// Try to send a heartbeat to force a reconnect to the replica group.
	go c.sendHeartbeat()
}

// Get a connection to the server. If it isn't possible to make a connection,
// return an error.
func (c *SessClnt) getConn() (*netclnt.NetClnt, *serr.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, serr.NewErr(serr.TErrUnreachable, c.addrs)
	}

	if c.nc == nil {
		db.DPrintf(db.SESSCLNT, "%v SessionConn reconnecting to %v %v\n", c.sid, c.addrs, c.closed)
		nc, err := netclnt.NewNetClnt(c.clntnet, c.addrs, spcodec.ReadCall, spcodec.WriteCall, c)
		if err != nil {
			db.DPrintf(db.SESSCLNT, "%v Error %v unable to reconnect to %v\n", c.sid, err, c.addrs)
			return nil, err
		}
		db.DPrintf(db.SESSCLNT, "%v Successful connection to %v out of %v\n", c.sid, nc.Dst(), c.addrs)
		c.nc = nc
	}
	return c.nc, nil
}

func (c *SessClnt) ownClosed() bool {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return false
	}
	c.closed = true
	return true
}

// Creator of sessclnt closes session
func (c *SessClnt) Close() error {
	return c.close()
}

// Close the session permanently
func (c *SessClnt) close() error {
	db.DPrintf(db.SESSCLNT, "%v Close session to %v %v\n", c.sid, c.addrs, c.closed)
	if !c.ownClosed() {
		return nil
	}
	nc := c.ownNetClnt()
	if nc == nil {
		return nil
	}
	return nc.Close()

	//outstanding := c.queue.Close()
	// Kill outstanding requests.
	//for _, rpc := range outstanding {
	//	rpc.Abort()
	//}
	//return nil
}
