// The sessclnt package establishes a session with a server using a
// [netclnt], which sets up a TCP connection.  If [netclnt] fails,
// sessclnt will return error, and sessclnt creates a new [netclnt] on
// the next RPC.  Sessclnt uses [demux] to multiplex requests/replies
// over the connetion.
package clnt

import (
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	netclnt "sigmaos/net/clnt"
	"sigmaos/proc"
	"sigmaos/serr"
	spcodec "sigmaos/session/codec"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
	"sigmaos/util/rand"
)

type SessClnt struct {
	sync.Mutex
	sid     sessp.Tsession
	seqcntr *sessp.Tseqcntr
	closed  bool
	ep      *sp.Tendpoint
	npc     *dialproxyclnt.DialProxyClnt
	nc      *netclnt.NetClnt
	pe      *proc.ProcEnv
	dmx     *demux.DemuxClnt
}

func newSessClnt(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, ep *sp.Tendpoint) (*SessClnt, *serr.Err) {
	c := &SessClnt{
		sid:     sessp.Tsession(rand.Uint64()),
		npc:     npc,
		pe:      pe,
		ep:      ep,
		seqcntr: new(sessp.Tseqcntr),
	}
	db.DPrintf(db.SESSCLNT, "Make session %v to srvs %v", c.sid, ep)
	if _, err := c.getConn(); err != nil {
		return nil, err
	}
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

func (c *SessClnt) resetNetClnt(nc *netclnt.NetClnt) {
	c.Lock()
	defer c.Unlock()
	if nc == c.nc {
		c.nc = nil
	}
}

func (c *SessClnt) IsConnected() bool {
	if nc := c.netClnt(); nc != nil {
		return !c.dmx.IsClosed()
	}
	return false
}

func (c *SessClnt) RPC(req sessp.Tmsg, iniov sessp.IoVec, outiov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	s := time.Now()
	fc := sessp.NewFcallMsg(req, iniov, c.sid, c.seqcntr)
	pmfc := spcodec.NewPartMarshaledMsg(fc)
	nc := c.netClnt()
	if nc == nil {
		if nc0, err := c.getConn(); err != nil {
			return nil, err
		} else {
			nc = nc0
		}
	}
	if db.WillBePrinted(db.SESSCLNT) {
		db.DPrintf(db.SESSCLNT, "sess %v RPC req %v", c.sid, fc)
	}
	rep, err := c.dmx.SendReceive(pmfc, outiov)
	if db.WillBePrinted(db.SESSCLNT) {
		db.DPrintf(db.SESSCLNT, "sess %v RPC req %v rep %v err %v", c.sid, fc, rep, err)
	}

	if err != nil {
		if err.IsErrUnreachable() {
			c.resetNetClnt(nc)
		}
		return nil, err
	}

	r := rep.(*sessp.PartMarshaledMsg)
	if err := spcodec.UnmarshalMsg(r); err != nil {
		return nil, err
	}
	db.DPrintf(db.NET_LAT, "RPC req %v fm %v lat %v\n", fc, r.Fcm, time.Since(s))
	return r.Fcm, nil
}

// For supporting reconnect
func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(sp.NewTheartbeat(map[uint64]bool{uint64(c.sid): true}), nil, nil)
	if err != nil {
		db.DPrintf(db.SESSCLNT_ERR, "%v heartbeat %v err %v", c.sid, c.ep, err)
	}
}

// Get a connection to the server and demux it with [demux]
func (c *SessClnt) getConn() (*netclnt.NetClnt, *serr.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, serr.NewErr(serr.TErrUnreachable, c.ep)
	}

	if c.nc == nil {
		db.DPrintf(db.SESSCLNT, "%v Connect to %v %v\n", c.sid, c.ep, c.closed)
		nc, err := netclnt.NewNetClnt(c.pe, c.npc, c.ep)
		if err != nil {
			db.DPrintf(db.SESSCLNT, "%v Error %v unable to reconnect to %v\n", c.sid, err, c.ep)
			return nil, err
		}
		db.DPrintf(db.SESSCLNT, "%v connection to %v out of %v\n", c.sid, nc.Dst(), c.ep)
		c.nc = nc
		iovm := demux.NewIoVecMap()
		c.dmx = demux.NewDemuxClnt(spcodec.NewTransport(nc.Conn(), iovm), iovm)
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
	db.DPrintf(db.SESSCLNT, "%v Close session to %v %v\n", c.sid, c.ep, c.closed)
	if !c.ownClosed() {
		return nil
	}
	nc := c.ownNetClnt()
	if nc == nil {
		return nil
	}
	// Close connection. This also causes dmxclnt to be closed
	return nc.Close()
}
