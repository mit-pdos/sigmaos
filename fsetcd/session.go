package fsetcd

import (
	// "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	sp "sigmaos/sigmap"
)

type Session struct {
	*concurrency.Session
}

func (fs *FsEtcd) NewSession() (*Session, error) {
	s, err := concurrency.NewSession(fs.Clnt(), concurrency.WithTTL(sp.EtcdSessionTTL))
	if err != nil {
		return nil, err
	}
	return &Session{s}, nil
}

func (sess *Session) Lease() sp.TleaseId {
	return sp.TleaseId(sess.Session.Lease())
}
