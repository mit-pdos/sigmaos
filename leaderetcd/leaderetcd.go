package leaderetcd

import (
	"context"

	"go.etcd.io/etcd/client/v3/concurrency"

	"sigmaos/namesrv/fsetcd"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type Election struct {
	sess *fsetcd.Session
	pn   string
	*concurrency.Election
	pe *proc.ProcEnv
}

func NewElection(pe *proc.ProcEnv, s *fsetcd.Session, pn string) (*Election, error) {
	el := &Election{sess: s, pn: pn, pe: pe}
	return el, nil
}

func (el *Election) Candidate() error {
	db.DPrintf(db.LEADER, "candidate %v %v\n", el.pe.GetPID().String(), el.pn)

	el.Election = concurrency.NewElection(el.sess.Session, el.pn)

	// XXX stick fence's sequence number in val?
	if err := el.Campaign(context.TODO(), el.pe.GetPID().String()); err != nil {
		return err
	}

	resp, err := el.Leader(context.TODO())
	if err != nil {
		return err
	}

	db.DPrintf(db.LEADER, "leader %v %v\n", el.pe.GetPID().String(), resp)
	return nil
}

func (el Election) Fence() sp.Tfence {
	return sp.NewFence(el.Election.Key(), sp.Tepoch(el.Election.Rev()))
}

func (el *Election) Resign() error {
	db.DPrintf(db.LEADER, "leader %v resign %v\n", el.pe.GetPID().String(), el.pn)
	return el.Election.Resign(context.TODO())
}
