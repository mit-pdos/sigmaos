// The sessclnt package establishes a session with a server using a
// [netclnt], which sets up a TCP connection.  If [netclnt] fails,
// sessclnt could re-establish a new [netclnt] (but no longer
// supported for now).  Sessclnt uses [demux] to multiplex
// requests/replies over the connetion.
package sessclnt

import (
	"sync"
	//"time"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netclnt"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type SessClnt struct {
	sync.Mutex
	sid     sessp.Tsession
	seqcntr *sessp.Tseqcntr
	closed  bool
	mnt     *sp.Tmount
	npc     *netsigma.NetProxyClnt
	nc      *netclnt.NetClnt
	pe      *proc.ProcEnv
	dmx     *demux.DemuxClnt
}

func newSessClnt(pe *proc.ProcEnv, npc *netsigma.NetProxyClnt, mnt *sp.Tmount) (*SessClnt, *serr.Err) {
	c := &SessClnt{
		sid:     sessp.Tsession(rand.Uint64()),
		npc:     npc,
		pe:      pe,
		mnt:     mnt,
		seqcntr: new(sessp.Tseqcntr),
	}
	db.DPrintf(db.SESSCLNT, "Make session %v to srvs %v", c.sid, mnt)
	if err := c.getConn(); err != nil {
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

func (c *SessClnt) IsConnected() bool {
	if nc := c.netClnt(); nc != nil {
		return !c.dmx.IsClosed()
	}
	return false
}

func (c *SessClnt) RPC(req sessp.Tmsg, iniov sessp.IoVec, outiov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	fc := sessp.NewFcallMsg(req, iniov, c.sid, c.seqcntr)
	pmfc := spcodec.NewPartMarshaledMsg(fc)
	nc := c.netClnt()
	if nc == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, c.mnt)
	}
	db.DPrintf(db.SESSCLNT, "sess %v RPC req %v", c.sid, fc)
	rep, err := c.dmx.SendReceive(pmfc, outiov)
	db.DPrintf(db.SESSCLNT, "sess %v RPC req %v rep %v err %v", c.sid, fc, rep, err)

	if err != nil {
		return nil, err
	}

	r := rep.(*sessp.PartMarshaledMsg)
	if err := spcodec.UnmarshalMsg(r); err != nil {
		return nil, err
	}

	return r.Fcm, nil
}

// For supporting reconnect
func (c *SessClnt) sendHeartbeat() {
	_, err := c.RPC(sp.NewTheartbeat(map[uint64]bool{uint64(c.sid): true}), nil, nil)
	if err != nil {
		db.DPrintf(db.SESSCLNT_ERR, "%v heartbeat %v err %v", c.sid, c.mnt, err)
	}
}

// Get a connection to the server and demux it with [demux]
func (c *SessClnt) getConn() *serr.Err {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return serr.NewErr(serr.TErrUnreachable, c.mnt)
	}

	if c.nc == nil {
		db.DPrintf(db.SESSCLNT, "%v Connect to %v %v\n", c.sid, c.mnt, c.closed)
		nc, err := netclnt.NewNetClnt(c.pe, c.npc, c.mnt)
		if err != nil {
			db.DPrintf(db.SESSCLNT, "%v Error %v unable to reconnect to %v\n", c.sid, err, c.mnt)
			return err
		}
		db.DPrintf(db.SESSCLNT, "%v connection to %v out of %v\n", c.sid, nc.Dst(), c.mnt)
		c.nc = nc
		iovm := demux.NewIoVecMap()
		c.dmx = demux.NewDemuxClnt(spcodec.NewTransport(nc.Conn(), iovm), iovm)
	}
	return nil
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
	db.DPrintf(db.SESSCLNT, "%v Close session to %v %v\n", c.sid, c.mnt, c.closed)
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
