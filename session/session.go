package session

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	"ulambda/lease"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

//
// A session identifies a client across TCP connections.  For each
// session, sigmaos has a protsrv.
//

type Session struct {
	sync.Mutex
	protsrv protsrv.Protsrv
	lease   *lease.Lease
}

func makeSession(protsrv protsrv.Protsrv) *Session {
	sess := &Session{}
	sess.protsrv = protsrv
	return sess
}

func (sess *Session) Lease(sid np.Tsession, fn []string, qid np.Tqid) error {
	sess.Lock()
	defer sess.Unlock()

	if sess.lease != nil {
		return fmt.Errorf("%v: lease present already %v", db.GetName(), sid)
	}

	sess.lease = lease.MakeLease(fn, qid)
	return nil
}

func (sess *Session) Unlease(sid np.Tsession, fn []string) error {
	sess.Lock()
	defer sess.Unlock()

	if sess.lease == nil {
		return fmt.Errorf("%v: Lease no lease %v", db.GetName(), sid)
	}

	if !np.IsPathEq(sess.lease.Fn, fn) {
		return fmt.Errorf("%v: Lease lease is for %v not %v", db.GetName(), sess.lease.Fn, fn)
	}

	sess.lease = nil
	return nil
}

func (sess *Session) CheckLease(fn []string, qid np.Tqid) error {
	sess.Lock()
	defer sess.Unlock()

	if sess.lease == nil {
		return fmt.Errorf("%v: CheckLease no lease", db.GetName())
	}

	if !np.IsPathEq(sess.lease.Fn, fn) {
		return fmt.Errorf("%v: CheckLease lease is for %v not %v", db.GetName(), sess.lease.Fn, fn)
	}
	return sess.lease.Check(qid)
}

func (sess *Session) LeaseName() ([]string, error) {
	sess.Lock()
	defer sess.Unlock()
	if sess.lease == nil {
		return nil, nil
	}
	return sess.lease.Fn, nil
}
