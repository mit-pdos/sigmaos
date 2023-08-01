package leaderetcd

import (
	"context"

	// "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"sigmaos/fsetcd"

	db "sigmaos/debug"
	"sigmaos/proc"
)

type Election struct {
	sess *fsetcd.Session
	pn   string
	*concurrency.Election
}

func MkElection(s *fsetcd.Session, pn string) (*Election, error) {
	el := &Election{sess: s, pn: pn}
	return el, nil
}

func (el *Election) Candidate() error {
	db.DPrintf(db.LEADER, "candidate %v %v\n", proc.GetPid().String(), el.pn)

	el.Election = concurrency.NewElection(el.sess.Session, el.pn)

	// XXX stick fence's sequence number in val?
	if err := el.Campaign(context.TODO(), proc.GetPid().String()); err != nil {
		return err
	}

	resp, err := el.Leader(context.TODO())
	if err != nil {
		return err
	}

	db.DPrintf(db.LEADER, "leader %v %v\n", proc.GetPid().String(), resp)
	return nil
}

func (el *Election) Resign() error {
	db.DPrintf(db.LEADER, "leader %v resign %v\n", proc.GetPid().String(), el.pn)
	return el.Election.Resign(context.TODO())

}
