package session

import (
	"log"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/protsrv"
	"ulambda/threadmgr"
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
	wg            sync.WaitGroup
	conn          protsrv.NetConn
	protsrv       protsrv.Protsrv
	lastHeartbeat time.Time
	Sid           np.Tsession
	began         bool // true if the fssrv has already begun processing ops
	running       bool // true if the session is currently running an operation.
	closed        bool // true if the session has been closed.
	replies       chan *np.Fcall
	timedout      bool // for debugging
}

func makeSession(conn protsrv.NetConn, protsrv protsrv.Protsrv, sid np.Tsession, replies chan *np.Fcall, t *threadmgr.ThreadMgr) *Session {
	sess := &Session{}
	sess.threadmgr = t
	sess.conn = conn
	sess.protsrv = protsrv
	sess.lastHeartbeat = time.Now()
	sess.Sid = sid
	sess.replies = replies
	sess.lastHeartbeat = time.Now()
	return sess
}

func (sess *Session) GetRepliesC() chan *np.Fcall {
	sess.Lock()
	defer sess.Unlock()
	return sess.replies
}

func (sess *Session) GetThread() *threadmgr.ThreadMgr {
	return sess.threadmgr
}

func (sess *Session) IncThreads() {
	sess.wg.Add(1)
}

func (sess *Session) DecThreads() {
	sess.wg.Done()
}

func (sess *Session) WaitThreads() {
	sess.wg.Wait()
}

// For testing. Invoking CloseConn() will eventually cause
// sess.Close() to be called.
func (sess *Session) CloseConn() {
	sess.conn.Close()
}

func (sess *Session) Close() {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		log.Fatalf("FATAL tried to close a closed session: %v", sess.Sid)
	}
	sess.closed = true
	// Close the replies channel so that writer in srvconn exits
	if sess.replies != nil {
		db.DLPrintf("SESSION", "%v close replies\n", sess.Sid)
		close(sess.replies)
		sess.replies = nil
	}
}

func (sess *Session) IsClosed() bool {
	sess.Lock()
	defer sess.Unlock()
	return sess.closed
}

// Change the replies channel if the new channel is non-nil. This may occur if,
// for example, a client starts talking to a new replica.
func (sess *Session) maybeSetConn(conn protsrv.NetConn, replies chan *np.Fcall) {
	sess.Lock()
	defer sess.Unlock()
	if conn != nil { // maybe nil
		if sess.conn != conn {
			db.DLPrintf("SESSION", "maybeSetConn new %v\n", conn)
			sess.conn = conn
		}
	}
	if replies != nil {
		sess.replies = replies
	}
}

func (sess *Session) heartbeat(msg np.Tmsg) {
	sess.Lock()
	defer sess.Unlock()
	db.DLPrintf("SESSION", "Heartbeat %v %v", msg.Type(), msg)
	if sess.closed {
		log.Fatalf("FATAL %v heartbeat %v on closed session %v", proc.GetName(), msg, sess.Sid)
	}
	sess.lastHeartbeat = time.Now()
}

// Indirectly timeout a session
func (sess *Session) timeout() {
	sess.Lock()
	defer sess.Unlock()
	db.DLPrintf("SESSION0", "timeout %v", sess.Sid)
	sess.timedout = true
}

func (sess *Session) timedOut() bool {
	sess.Lock()
	defer sess.Unlock()
	// If in the middle of a running op, or this fssrv hasn't begun processing
	// ops yet, refresh the heartbeat so we don't immediately time-out when the
	// op finishes.
	if sess.running || !sess.began {
		sess.lastHeartbeat = time.Now()
		return false
	}
	return sess.timedout || time.Since(sess.lastHeartbeat).Milliseconds() > np.SESSTIMEOUTMS
}

func (sess *Session) SetRunning(r bool) {
	sess.Lock()
	defer sess.Unlock()
	sess.running = r
	// If this server is replicated, it may take a couple of seconds for the
	// replication library to start up & begin processing ops. Noting when
	// processing has started for a session helps us avoid timing-out sessions
	// until they have begun processing ops.
	sess.began = true
}
