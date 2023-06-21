package fsetcd_test

import (
	"context"
	"log"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	sp "sigmaos/sigmap"
)

func TestLease(t *testing.T) {
	ec, err := fsetcd.MkEtcdClnt(sp.ROOTREALM)
	assert.Nil(t, err)
	l := clientv3.NewLease(ec.Client)
	respg, err := l.Grant(context.TODO(), 30)
	assert.Nil(t, err)
	log.Printf("resp %x %v\n", respg.ID, respg.TTL)
	respl, err := l.Leases(context.TODO())
	log.Printf("resp %v\n", respl.Leases)
	respttl, err := l.TimeToLive(context.TODO(), respg.ID)
	log.Printf("resp %v\n", respttl.TTL)
	ch, err := l.KeepAlive(context.TODO(), respg.ID)
	go func() {
		for respa := range ch {
			log.Printf("respa %v\n", respa.TTL)
		}
	}()
	time.Sleep(60 * time.Second)

	err = l.Close()
	assert.Nil(t, err)
}
