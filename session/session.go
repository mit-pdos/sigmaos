package session

import (
	"log"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fences"
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
	protsrv       protsrv.Protsrv
	rft           *fences.RecentTable
	myFences      *fences.FenceTable
	lastHeartbeat time.Time
	Sid           np.Tsession
	Running       bool // true if the session is currently running an operation.
	closed        bool // true if the session has been closed.
	replies       chan *np.Fcall
}

func makeSession(protsrv protsrv.Protsrv, sid np.Tsession, replies chan *np.Fcall, rft *fences.RecentTable, t *threadmgr.ThreadMgr) *Session {
	sess := &Session{}
	sess.threadmgr = t
	sess.protsrv = protsrv
	sess.lastHeartbeat = time.Now()
	sess.Sid = sid
	sess.rft = rft
	sess.myFences = fences.MakeFenceTable()
	sess.replies = replies
	return sess
}

func (sess *Session) GetRepliesC() chan *np.Fcall {
	sess.Lock()
	defer sess.Unlock()
	return sess.replies
}

func (sess *Session) Fence(pn []string, fence np.Tfence) {
	sess.myFences.Insert(pn, fence)
}

func (sess *Session) GetThread() *threadmgr.ThreadMgr {
	return sess.threadmgr
}

func (sess *Session) Unfence(path []string, idf np.Tfenceid) *np.Err {
	return sess.myFences.Del(path, idf)
}

func (sess *Session) CheckFences(path []string) *np.Err {
	fences := sess.myFences.Fences(path)
	for _, f := range fences {
		err := sess.rft.IsRecent(f)
		if err != nil {
			db.DLPrintf("FENCE", "%v: fence %v err %v\n", proc.GetName(), path, err)
			return err
		}
	}
	return nil
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

func (sess *Session) Close() {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		log.Fatalf("FATAL tried to close a closed session: %v", sess.Sid)
	}
	sess.closed = true
}

func (sess *Session) IsClosed() bool {
	sess.Lock()
	defer sess.Unlock()
	return sess.closed
}

// Change the replies channel if the new channel is non-nil. This may occur if,
// for example, a client starts talking to a new replica.
func (sess *Session) maybeSetRepliesC(replies chan *np.Fcall) {
	sess.Lock()
	defer sess.Unlock()
	if replies != nil {
		sess.replies = replies
	}
}

func (sess *Session) heartbeat() {
	sess.Lock()
	defer sess.Unlock()
	if sess.closed {
		log.Fatalf("FATAL heartbeat on closed session %v", sess.Sid)
	}
	sess.lastHeartbeat = time.Now()
}

func (sess *Session) timedOut() bool {
	sess.Lock()
	defer sess.Unlock()
	return !sess.Running && time.Since(sess.lastHeartbeat).Milliseconds() > SESSTIMEOUTMS
}
