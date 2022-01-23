package session

import (
	"fmt"
	"log"
	"sync"

	"github.com/sasha-s/go-deadlock"

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

type Session struct {
	deadlock.Mutex // to serialize requests of a session
	cond           *sync.Cond
	Nthread        int
	Nblocked       int
	protsrv        protsrv.Protsrv
	lm             *lease.LeaseMap
	sid            np.Tsession
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

func (sess *Session) Inc() {
	sess.Nthread += 1
}

func (sess *Session) Dec() {
	sess.Nthread -= 1
	sess.cond.Signal()
}

func (sess *Session) WaitTotalZero() {
	for sess.Nthread > 0 {
		log.Printf("%v: wait T nthread %v nblocked %v\n", sess.sid, sess.Nthread, sess.Nblocked)
		sess.cond.Wait()
	}
}
