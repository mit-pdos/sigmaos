package leaderetcd

import (
	"context"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"sigmaos/fsetcd"

	db "sigmaos/debug"
	"sigmaos/proc"
)

type Election struct {
	*clientv3.Client
	sess *concurrency.Session
	pn   string
	*concurrency.Election
}

func MkElection(ec *clientv3.Client, pn string) (*Election, error) {
	el := &Election{Client: ec, pn: pn}

	s, err := concurrency.NewSession(ec, concurrency.WithTTL(fsetcd.SessionTTL))
	if err != nil {
		return nil, err
	}
	el.sess = s
	return el, nil
}

func (el *Election) Candidate() error {
	db.DPrintf(db.LEADER, "candidate %v %v\n", proc.GetPid().String(), el.pn)

	el.Election = concurrency.NewElection(el.sess, el.pn)

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
