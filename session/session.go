package session

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	"ulambda/fences"
	np "ulambda/ninep"
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
	threadmgr *threadmgr.ThreadMgr
	wg        sync.WaitGroup
	protsrv   protsrv.Protsrv
	rft       *fences.RecentTable
	myFences  *fences.FenceTable
	Sid       np.Tsession
}

func makeSession(protsrv protsrv.Protsrv, sid np.Tsession, rft *fences.RecentTable, t *threadmgr.ThreadMgr) *Session {
	sess := &Session{}
	sess.threadmgr = t
	sess.protsrv = protsrv
	sess.Sid = sid
	sess.rft = rft
	sess.myFences = fences.MakeFenceTable()
	return sess
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
	//if len(fences) > 0 {
	//	log.Printf("%v: CheckFences %v %v\n", sess.Sid, path, fences)
	//}
	for _, f := range fences {
		err := sess.rft.IsRecent(f)
		if err != nil {
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
