package sessstatesrv

import (
	"fmt"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessconn"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
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
	conn          sps.Conn
	protsrv       sps.Protsrv
	lastHeartbeat time.Time
	Sid           sessp.Tsession
	began         bool // true if the fssrv has already begun processing ops
	closed        bool // true if the session has been closed.
	timedout      bool // for debugging
	attachClnt    sps.AttachClntF
	detachSess    sps.DetachSessF
	detachClnt    sps.DetachClntF
	clnts         map[sp.TclntId]bool
}

func newSession(protsrv sps.Protsrv, sid sessp.Tsession, attachf sps.AttachClntF, detachf sps.DetachClntF) *Session {
	sess := &Session{
		protsrv:       protsrv,
		lastHeartbeat: time.Now(),
		Sid:           sid,
		attachClnt:    attachf,
		detachClnt:    detachf,
		clnts:         make(map[sp.TclntId]bool),
	}
	return sess
}

// XXX reimplement
func (sess *Session) QueueLen() int64 {
	return 0
}

func (sess *Session) GetConn() sps.Conn {
	sess.Lock()
	defer sess.Unlock()
	return sess.conn
}

// For testing. Invoking CloseConn() will eventually cause
// sess.Close() to be called by Detach().
func (sess *Session) CloseConn() {
	sess.Lock()
	var conn sps.Conn
	if sess.conn != nil {
		conn = sess.conn
	}
	sess.Unlock()
	conn.CloseConnTest()
}

func (sess *Session) AddClnt(cid sp.TclntId) {
	sess.Lock()
	defer sess.Unlock()
	sess.clnts[cid] = true
}

func (sess *Session) CloseClnt(cid sp.TclntId) {
	sess.Lock()
	defer sess.Unlock()
	delete(sess.clnts, cid)
}

// Eagerly close the session if not in use
func (sess *Session) CheckForClose() {
	sess.Lock()
	defer sess.Unlock()
	if len(sess.clnts) == 0 && sess.closed == false {
		sess.close()
	}
}

// Server may call Close() several times because client may reconnect
// on a session that server has terminated and the Close() will close
// the new reply channel.
func (sess *Session) close() {
	db.DPrintf(db.ALWAYS, "Close session %v\n", sess.Sid)
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

// If the conn is nil, a reply is not needed. Conn maybe be nil
// because server closed session unilaterally.
func (sess *Session) SendConn(fm *sessp.FcallMsg) {
	var replies chan *sessconn.PartMarshaledMsg

	sess.Lock()
	if sess.conn != nil {
		// Must get replies channel under lock. This ensures that the connection's
		// WaitGroup is added to before the connection is closed, and ensures the
		// replies channel isn't closed from under our feet.
		replies = sess.conn.GetReplyChan()
	}
	sess.Unlock()

	// If there was a connection associated with this session...
	if replies != nil {
		replies <- sessconn.NewPartMarshaledMsg(fm)
	}
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
func (sess *Session) SetConn(conn sps.Conn) *serr.Err {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		return serr.NewErr(serr.TErrClosed, fmt.Sprintf("session %v", sess.Sid))
	}
	db.DPrintf(db.SESS_STATE_SRV, "%v SetConn new %v\n", sess.Sid, conn)
	sess.conn = conn
	return nil
}

func (sess *Session) UnsetConn(conn sps.Conn) {
	sess.Lock()
	defer sess.Unlock()

	sess.unsetConnL(conn)
}

// Disassociate a connection with this session, and safely close the
// connection.
func (sess *Session) unsetConnL(conn sps.Conn) {
	if sess.conn == conn {
		db.DPrintf(db.SESS_STATE_SRV, "%v close connection", sess.Sid)
		sess.conn = nil
	}
	conn.Close()
}

// Caller holds lock.
func (sess *Session) heartbeatL(msg sessp.Tmsg) {
	db.DPrintf(db.SESS_STATE_SRV, "Heartbeat sess %v msg %v %v", sess.Sid, msg.Type(), msg)
	if sess.closed {
		db.DFatalf("heartbeat %v on closed session %v", msg, sess.Sid)
	}
	sess.lastHeartbeat = time.Now()
}

// Indirectly timeout a session
func (sess *Session) timeout() {
	sess.Lock()
	defer sess.Unlock()
	db.DPrintf(db.SESS_STATE_SRV, "timeout %v", sess.Sid)
	sess.timedout = true
}

func (sess *Session) isConnected() bool {
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

func (sess *Session) RegisterDetachClnt(f sps.DetachClntF) {
	sess.Lock()
	defer sess.Unlock()
	sess.detachClnt = f
}
