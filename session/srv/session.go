package srv

import (
	"fmt"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	sps "sigmaos/api/spprotsrv"
	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

//
// A session identifies a client across TCP connections.  For each
// session, sigmaos has a protsrv.
//
// The sess lock is to serialize requests on a session.  The calls in
// this file assume the calling wg holds the sess lock.
//
//

type Session struct {
	sync.Mutex
	conn          *netConn
	protsrv       sps.ProtSrv
	lastHeartbeat time.Time
	Sid           sessp.Tsession
	began         bool // true if the fssrv has already begun processing ops
	closed        bool // true if the session has been closed.
	timedout      bool // for debugging
	detachSess    sps.DetachSessF
	clnts         map[sp.TclntId]bool
}

func newSession(protsrv sps.ProtSrv, sid sessp.Tsession, nc *netConn) *Session {
	sess := &Session{
		protsrv:       protsrv,
		lastHeartbeat: time.Now(),
		Sid:           sid,
		conn:          nc,
		clnts:         make(map[sp.TclntId]bool),
	}
	return sess
}

// XXX reimplement
func (sess *Session) QueueLen() int64 {
	return 0
}

func (sess *Session) GetConn() *netConn {
	sess.Lock()
	defer sess.Unlock()

	return sess.conn
}

// For testing. Invoking CloseConn() will eventually cause
// sess.Close() to be called by Detach(). Client will retry & re-establish
// connection.
func (sess *Session) CloseConn() {
	sess.Lock()
	var conn *netConn
	if sess.conn != nil {
		conn = sess.conn
	}
	sess.Unlock()
	if conn != nil {
		conn.Close()
	}
}

func (sess *Session) AddClnt(cid sp.TclntId) {
	sess.Lock()
	defer sess.Unlock()
	db.DPrintf(db.SESSSRV, "Add cid %v sess %v %d\n", cid, sess.Sid, len(sess.clnts))
	sess.clnts[cid] = true
}

// Delete client from session
func (sess *Session) DelClnt(cid sp.TclntId) {
	sess.Lock()
	defer sess.Unlock()
	delete(sess.clnts, cid)
	db.DPrintf(db.SESSSRV, "Del cid %v sess %v %d\n", cid, sess.Sid, len(sess.clnts))
}

// Server may call Close() several times because client may reconnect
// on a session that server has terminated and the Close() will close
// the new reply channel.
func (sess *Session) close() {
	db.DPrintf(db.SESSSRV, "Srv Close sess %v %d\n", sess.Sid, len(sess.clnts))
	sess.closed = true
	// Close the connection so that writer in srvconn exits
	if sess.conn != nil {
		sess.unsetConnL(sess.conn)
	}
}

func (sess *Session) Close() {
	sess.Lock()
	defer sess.Unlock()
	sess.close()
}

func (sess *Session) getClnts() []sp.TclntId {
	cs := make([]sp.TclntId, 0, len(sess.clnts))
	for c, _ := range sess.clnts {
		cs = append(cs, c)
	}
	return cs
}

func (sess *Session) IsClosed() bool {
	sess.Lock()
	defer sess.Unlock()
	return sess.closed
}

// Change conn associated with this session. This may occur if, for example, a
// client starts client reconnects quickly.
func (sess *Session) SetConn(conn *netConn) *serr.Err {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		return serr.NewErr(serr.TErrClosed, fmt.Sprintf("sess %v", sess.Sid))
	}
	db.DPrintf(db.SESSSRV, "%v SetConn new %v\n", sess.Sid, conn)
	sess.conn = conn
	return nil
}

func (sess *Session) UnsetConn(nc *netConn) {
	sess.Lock()
	defer sess.Unlock()

	sess.unsetConnL(nc)
}

// Disassociate a connection with this session, and safely close the
// connection.
func (sess *Session) unsetConnL(nc *netConn) {
	if sess.conn == nc {
		db.DPrintf(db.SESSSRV, "%v close connection", sess.Sid)
		sess.conn = nil
	}
	nc.Close()
}

func (sess *Session) IsConnected() bool {
	sess.Lock()
	defer sess.Unlock()

	if sess.closed || sess.conn == nil || sess.conn.IsClosed() {
		return false
	}
	return true
}

func (sess *Session) timedOut() (bool, time.Time) {
	sess.Lock()
	defer sess.Unlock()
	// For testing purposes.
	if sess.timedout {
		return true, sess.lastHeartbeat
	}
	return sess.timedout || time.Since(sess.lastHeartbeat) > sp.Conf.Session.TIMEOUT, sess.lastHeartbeat
}

func (sess *Session) RegisterDetachSess(f sps.DetachSessF) {
	sess.Lock()
	defer sess.Unlock()
	sess.detachSess = f
}

func (sess *Session) GetDetachSess() sps.DetachSessF {
	sess.Lock()
	defer sess.Unlock()
	return sess.detachSess
}

func (s *Session) Dispatch(msg sessp.Tmsg, iov sessp.IoVec) (sessp.Tmsg, sessp.IoVec, *sp.Rerror, sps.Tsessop, sp.TclntId) {
	if s.IsClosed() {
		db.DPrintf(db.SESSSRV, "Sess %v is closed; reject %v\n", s.Sid, msg.Type())
		err := serr.NewErr(serr.TErrClosed, fmt.Sprintf("session %v", s.Sid))
		return nil, nil, sp.NewRerrorSerr(err), sps.TSESS_NONE, sp.NoClntId
	}
	return sps.Dispatch(s.protsrv, msg, iov)
}
