// The sessclnt package establishes a session server using a TCP
// connection.  If TCP connection fails, it will try to re-establish
// the connection.
package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	//"time"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/netclnt"
	"sigmaos/rand"
	"sigmaos/serr"
	//"sigmaos/sessconn"
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

func newSessClnt(clntnet string, addrs sp.Taddrs) (*SessClnt, *serr.Err) {
	c := &SessClnt{}
	c.sid = sessp.Tsession(rand.Uint64())
	c.seqno = 0
	c.addrs = addrs
	c.nc = nil
	c.clntnet = clntnet
	c.queue = NewRequestQueue(addrs)
	db.DPrintf(db.SESSCLNT, "Make session %v to srvs %v", c.sid, addrs)
	nc, err := netclnt.NewNetClnt(clntnet, addrs, c)
	if err != nil {
		return nil, err
	}
	c.nc = nc
	return c, nil
}

func (c *SessClnt) ReportError(err error) {
	db.DPrintf(db.SESSCLNT, "Netclnt reports err %v\n", err)
	if c.nc != nil {
		c.nc.Close()
	}
}

func (c *SessClnt) RPC(req sessp.Tmsg, data []byte) (*sessp.FcallMsg, *serr.Err) {
	fc := sessp.NewFcallMsg(req, data, c.sid, &c.seqno)
	r := make([]frame.Tframe, 2)
	r[0] = spcodec.MarshalFcallWithoutData(fc)
	r[1] = data
	rep, err := c.nc.SendReceive(r)
	if err != nil {
		return nil, err
	}
	fm := spcodec.UnmarshalFcallAndData(rep[0], rep[1])
	return fm, nil
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
		nc, err := netclnt.NewNetClnt(c.clntnet, c.addrs, c)
		if err != nil {
			db.DPrintf(db.SESSCLNT, "%v Error %v unable to reconnect to %v\n", c.sid, err, c.addrs)
			return nil, err
		}
		db.DPrintf(db.SESSCLNT, "%v Successful connection to %v out of %v\n", c.sid, nc.Dst(), c.addrs)
		c.nc = nc
	}
	return c.nc, nil
}

// Creator of sessclnt closes session
func (c *SessClnt) IsConnected() bool {
	if c.nc != nil {
		return !c.nc.IsClosed()
	}
	return false
}

// Creator of sessclnt closes session
func (c *SessClnt) Close() error {
	return c.close()
}

// Close the sessclnt connection
func (c *SessClnt) close() error {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	db.DPrintf(db.SESSCLNT, "%v Close session to %v %v\n", c.sid, c.addrs, c.closed)
	if c.nc != nil {
		c.nc.Close()
	}
	outstanding := c.queue.Close()
	// Kill outstanding requests.
	for _, rpc := range outstanding {
		rpc.Abort()
	}
	return nil
}
