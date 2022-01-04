package session

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
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
	leases  map[string]*lease.Lease
}

func makeSession(protsrv protsrv.Protsrv) *Session {
	sess := &Session{}
	sess.protsrv = protsrv
	sess.leases = make(map[string]*lease.Lease)
	return sess
}

func (sess *Session) Lease(sid np.Tsession, fn []string, qid np.Tqid) error {
	sess.Lock()
	defer sess.Unlock()

	f := np.Join(fn)
	if _, ok := sess.leases[f]; ok {
		return fmt.Errorf("%v: lease present already %v", db.GetName(), sid)
	}
	sess.leases[f] = lease.MakeLease(fn, qid)
	return nil
}

func (sess *Session) Unlease(sid np.Tsession, fn []string) error {
	sess.Lock()
	defer sess.Unlock()

	f := np.Join(fn)
	if _, ok := sess.leases[f]; !ok {
		return fmt.Errorf("%v: Lease no lease %v", db.GetName(), sid)
	}
	delete(sess.leases, f)
	return nil
}

func (sess *Session) CheckLeases(fsl *fslib.FsLib) error {
	sess.Lock()
	defer sess.Unlock()

	for fn, l := range sess.leases {
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
