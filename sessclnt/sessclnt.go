// The sessclnt package establishes a session with a server using a
// [netclnt], which sets up a TCP connection.  If [netclnt] fails,
// sessclnt could re-establish a new [netclnt] (but no longer
// supported for now).
package sessclnt

import (
	//	"github.com/sasha-s/go-deadlock"
	"sync"

	//"time"

	db "sigmaos/debug"
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
	clntnet string
}

func newSessClnt(clntnet string, addrs sp.Taddrs) (*SessClnt, *serr.Err) {
	c := &SessClnt{sid: sessp.Tsession(rand.Uint64()),
		clntnet: clntnet,
		addrs:   addrs,
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
// marshaling error, close conn?  If we want to support reconnect, we
// can get outstanding requests from dmxclnt.
func (c *SessClnt) ReportError(err error) {
	db.DPrintf(db.SESSCLNT, "Netclnt sess %v reports err %v\n", c.sid, err)
}

func (c *SessClnt) RPC(req sessp.Tmsg, iov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	fc := sessp.NewFcallMsg(req, iov, c.sid, &c.seqno)
	pmfc := spcodec.NewPartMarshaledMsg(fc)
	nc := c.netClnt()
	if nc == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, c.addrs)
	}
	rep, err := nc.SendReceive(pmfc)
	db.DPrintf(db.SESSCLNT, "sess %v RPC req %v rep %v err %v", c.sid, fc, rep, err)

	if err != nil {
		return nil, err
	}
	return rep.(*sessp.FcallMsg), nil
}

// For supporting reconnect
func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(sp.NewTheartbeat(map[uint64]bool{uint64(c.sid): true}), nil)
	if err != nil {
		db.DPrintf(db.SESSCLNT_ERR, "%v heartbeat %v err %v", c.sid, c.addrs, err)
	}
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

// Close the session permanently
func (c *SessClnt) Close() error {
	db.DPrintf(db.SESSCLNT, "%v Close session to %v %v\n", c.sid, c.addrs, c.closed)
	if !c.ownClosed() {
		return nil
	}
	nc := c.ownNetClnt()
	if nc == nil {
		return nil
	}
	return nc.Close()
}
