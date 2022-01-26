package session

import (
	"fmt"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/lease"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

//
// A session identifies a client across TCP connections.  For each
// session, sigmaos has a protsrv.
//
// The sess lock is to serialize requests on a session.  The calls in
// this file assume the calling threads holds the sess lock.
//

type Session struct {
	sync.Mutex // to serialize requests on a session
	cond       *sync.Cond
	threads    sync.WaitGroup
	protsrv    protsrv.Protsrv
	lm         *lease.LeaseMap
	sid        np.Tsession
}

func makeSession(protsrv protsrv.Protsrv, sid np.Tsession) *Session {
	sess := &Session{}
	sess.protsrv = protsrv
	sess.cond = sync.NewCond(&sess.Mutex)
	sess.lm = lease.MakeLeaseMap()
	sess.sid = sid
	return sess
}

func (sess *Session) Lease(fn []string, qid np.Tqid) error {
	return sess.lm.Add(lease.MakeLease(fn, qid))
}

func (sess *Session) Unlease(fn []string) error {
	return sess.lm.Del(fn)
}

func (sess *Session) CheckLeases(fsl *fslib.FsLib) error {
	leases := sess.lm.Leases()
	for _, l := range leases {
		fn := np.Join(l.Fn)
		st, err := fsl.Stat(fn)
		if err != nil {
			return fmt.Errorf("lease not found %v err %v", fn, err.Error())
		}
		err = l.Check(st.Qid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sess *Session) IncThreads() {
	sess.threads.Add(1)
}

func (sess *Session) DecThreads() {
	sess.threads.Done()
}

func (sess *Session) WaitThreads() {
	sess.threads.Wait()
}
