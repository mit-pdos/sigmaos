package session

import (
	"fmt"
	"sync"

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
	sync.Mutex
	protsrv protsrv.Protsrv
	lm      *lease.LeaseMap
}

func makeSession(protsrv protsrv.Protsrv) *Session {
	sess := &Session{}
	sess.protsrv = protsrv
	sess.lm = lease.MakeLeaseMap()
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
