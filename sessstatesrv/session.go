package sessstatesrv

import (
	"fmt"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/replies"
	"sigmaos/threadmgr"
)

//
// A session identifies a client across TCP connections.  For each
// session, sigmaos has a protsrv.
//
// The sess lock is to serialize requests on a session.  The calls in
// this file assume the calling wg holds the sess lock.
//

type Session struct {
	sync.Mutex
	threadmgr     *threadmgr.ThreadMgr
	conn          np.Conn
	rt            *replies.ReplyTable
	protsrv       np.Protsrv
	lastHeartbeat time.Time
	Sid           np.Tsession
	ClientId      np.Tclient
	began         bool // true if the fssrv has already begun processing ops
	closed        bool // true if the session has been closed.
	timedout      bool // for debugging
	detach        np.DetachF
}

func makeSession(protsrv np.Protsrv, cid np.Tclient, sid np.Tsession, t *threadmgr.ThreadMgr) *Session {
	sess := &Session{}
	sess.threadmgr = t
	sess.rt = replies.MakeReplyTable()
	sess.protsrv = protsrv
	sess.lastHeartbeat = time.Now()
	sess.Sid = sid
	sess.ClientId = cid
	sess.lastHeartbeat = time.Now()
	return sess
}

func (sess *Session) QueueLen() int {
	return sess.threadmgr.QueueLen()
}

func (sess *Session) GetReplyTable() *replies.ReplyTable {
	return sess.rt
}

func (sess *Session) GetConn() np.Conn {
	sess.Lock()
	defer sess.Unlock()
	return sess.conn
}

func (sess *Session) GetThread() *threadmgr.ThreadMgr {
	return sess.threadmgr
}

// For testing. Invoking CloseConn() will eventually cause
// sess.Close() to be called by Detach().
func (sess *Session) CloseConn() {
	sess.Lock()
	var conn np.Conn
	if sess.conn != nil {
		conn = sess.conn
	}
	sess.Unlock()
	conn.CloseConnTest()
}

// Server may call Close() several times because client may reconnect
// on a session that server has terminated and the Close() will close
// the new reply channel.
func (sess *Session) Close() {
	sess.Lock()
	defer sess.Unlock()
	db.DPrintf("SESSION", "Close session %v\n", sess.Sid)
	sess.closed = true
	// Close the connection so that writer in srvconn exits
	if sess.conn != nil {
		sess.unsetConnL(sess.conn)
	}
	// Empty & permanently close the replies table.
	sess.rt.Close(sess.ClientId, sess.Sid)
}

// The conn may be nil if this is a replicated op which came through
// raft; in this case, a reply is not needed. Conn maybe also be nil
// because server closed session unilaterally.
func (sess *Session) SendConn(fm *np.FcallMsg) {
	var replies chan *np.FcallMsg = nil

	sess.Lock()
	if sess.conn != nil {
		// Must get replies channel under lock. This ensures that the connection's
		// WaitGroup is added to before the connection is closed, and ensures the
		// replies channel isn't closed from under our feet.
		replies = sess.conn.GetReplyC()
	}
	sess.Unlock()

	// If there was a connection associated with this session...
	if replies != nil {
		replies <- fm
	}
}

func (sess *Session) IsClosed() bool {
	sess.Lock()
	defer sess.Unlock()
	return sess.closed
}

// Change conn associated with this session. This may occur if, for example, a
// client starts talking to a new replica or a client reconnects quickly.
func (sess *Session) SetConn(conn np.Conn) *np.Err {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		return np.MkErr(np.TErrClosed, fmt.Sprintf("session %v", sess.Sid))
	}
	db.DPrintf("SESSION", "%v SetConn new %v\n", sess.Sid, conn)
	sess.conn = conn
	return nil
}

func (sess *Session) UnsetConn(conn np.Conn) {
	sess.Lock()
	defer sess.Unlock()

	sess.unsetConnL(conn)
}

// Disassociate a connection with this session, and safely close the
// connection.
func (sess *Session) unsetConnL(conn np.Conn) {
	if sess.conn == conn {
		db.DPrintf("SESSION", "%v close connection", sess.Sid)
		sess.conn = nil
	}
	conn.Close()
}

// Caller holds lock.
func (sess *Session) heartbeatL(msg np.Tmsg) {
	db.DPrintf("SESSION", "Heartbeat sess %v msg %v %v", sess.Sid, msg.Type(), msg)
	if sess.closed {
		db.DFatalf("%v heartbeat %v on closed session %v", proc.GetName(), msg, sess.Sid)
	}
	sess.lastHeartbeat = time.Now()
}

// Indirectly timeout a session
func (sess *Session) timeout() {
	sess.Lock()
	defer sess.Unlock()
	db.DPrintf("SESSION", "timeout %v", sess.Sid)
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
	return sess.timedout || time.Since(sess.lastHeartbeat) > np.Conf.Session.TIMEOUT, sess.lastHeartbeat
}

func (sess *Session) RegisterDetach(f np.DetachF) {
	sess.Lock()
	defer sess.Unlock()
	sess.detach = f
}

func (sess *Session) GetDetach() np.DetachF {
	sess.Lock()
	defer sess.Unlock()
	return sess.detach
}
