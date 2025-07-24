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
	"sigmaos/util/spstats"
)

type SessClnt struct {
	sync.Mutex
	sid     sessp.Tsession
	seqcntr *sessp.Tseqcntr
	closed  bool
	ep      *sp.Tendpoint
	npc     *dialproxyclnt.DialProxyClnt
	pe      *proc.ProcEnv
	dmx     *demux.DemuxClnt
	pcst    *spstats.PathClntStats
}

func newSessClnt(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, ep *sp.Tendpoint, pcst *spstats.PathClntStats) (*SessClnt, *serr.Err) {
	c := &SessClnt{
		sid:     sessp.Tsession(rand.Uint64()),
		npc:     npc,
		pe:      pe,
		ep:      ep,
		pcst:    pcst,
		seqcntr: new(sessp.Tseqcntr),
	}
	db.DPrintf(db.SESSCLNT, "Make session %v to srvs %v", c.sid, ep)
	if _, err := c.getdmx(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *SessClnt) SessId() sessp.Tsession {
	return c.sid
}

func (c *SessClnt) resetdmx(dmx *demux.DemuxClnt) {
	c.Lock()
	defer c.Unlock()
	if dmx == c.dmx {
		c.dmx = nil
	}
}

func (c *SessClnt) IsConnected() bool {
	return !c.dmx.IsClosed()
}

func (c *SessClnt) RPC(req sessp.Tmsg, iniov sessp.IoVec, outiov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	s := time.Now()
	fc := sessp.NewFcallMsg(req, iniov, c.sid, c.seqcntr)
	pmfc := spcodec.NewPartMarshaledMsg(fc)
	marshalLat := time.Since(s)
	dmx, err := c.getdmx()
	if err != nil {
		return nil, err
	}
	if db.WillBePrinted(db.SESSCLNT) {
		db.DPrintf(db.SESSCLNT, "sess %v RPC req %v", c.sid, fc)
	}
	rep, err := dmx.SendReceive(pmfc, outiov)
	if db.WillBePrinted(db.SESSCLNT) {
		db.DPrintf(db.SESSCLNT, "sess %v RPC req %v rep %v err %v", c.sid, fc, rep, err)
	}

	if err != nil {
		if err.IsErrSession() {
			db.DPrintf(db.SESSCLNT_ERR, "Reset sess %v's dmx %p err %v", c.sid, dmx, err)
			c.resetdmx(dmx)
		}
		return nil, err
	}

	r := rep.(*sessp.PartMarshaledMsg)
	if err := spcodec.UnmarshalMsg(r); err != nil {
		return nil, err
	}
	db.DPrintf(db.NET_LAT, "RPC req %v fm %v marshalLat %v lat %v", fc, r.Fcm, marshalLat, time.Since(s))
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
func (c *SessClnt) getdmx() (*demux.DemuxClnt, *serr.Err) {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil, serr.NewErr(serr.TErrUnreachable, c.ep)
	}

	if c.dmx == nil {
		db.DPrintf(db.SESSCLNT, "%v Connect to %v %v", c.sid, c.ep, c.closed)
		conn, err := netclnt.NewNetClnt(c.pe, c.npc, c.ep)
		if err != nil {
			db.DPrintf(db.SESSCLNT, "%v Error %v unable to connect to %v", c.sid, err, c.ep)
			spstats.Inc(&c.pcst.NnetclntErr, 1)
			return nil, err
		}
		spstats.Inc(&c.pcst.NnetclntOK, 1)
		db.DPrintf(db.SESSCLNT, "%v connection to %v", c.sid, c.ep)
		iovm := demux.NewIoVecMap()
		c.dmx = demux.NewDemuxClnt(spcodec.NewTransport(conn, iovm), iovm)
	}
	return c.dmx, nil
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
	db.DPrintf(db.SESSCLNT, "%v Close session to %v %v", c.sid, c.ep, c.closed)
	if !c.ownClosed() {
		return nil
	}
	if c.dmx == nil {
		return nil
	}
	return c.dmx.Close()
}
