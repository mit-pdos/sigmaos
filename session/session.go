package session

import (
	"sync"

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
	threadmgr *threadmgr.ThreadMgr
	wg        sync.WaitGroup
	protsrv   protsrv.Protsrv
	rft       *fences.RecentTable
	myFences  *fences.FenceTable
	sm        *SessionMgr
	Sid       np.Tsession
	replies   chan *np.Fcall
}

func makeSession(protsrv protsrv.Protsrv, sid np.Tsession, replies chan *np.Fcall, rft *fences.RecentTable, t *threadmgr.ThreadMgr, sm *SessionMgr) *Session {
	sess := &Session{}
	sess.threadmgr = t
	sess.protsrv = protsrv
	sess.sm = sm
	sess.Sid = sid
	sess.rft = rft
	sess.myFences = fences.MakeFenceTable()
	sess.replies = replies
	// Register the new session.
	sess.sm.RegisterSession(sess.Sid)
	return sess
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
