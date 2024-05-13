package fsetcd_test

import (
	"context"
	"log"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestLease(t *testing.T) {
	lip := sp.Tip("127.0.0.1")
	_, _, amgr, err := test.NewAuthMgr()
	assert.Nil(t, err)
	secrets := map[string]*proc.ProcSecretProto{}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(amgr, sp.Tip(test.EtcdIP))
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, lip, lip, "", false, false, false, false)
	npc := netproxyclnt.NewNetProxyClnt(pe, nil)
	ec, err := fsetcd.NewFsEtcd(npc.Dial, pe.GetEtcdEndpoints(), pe.GetRealm())
	assert.Nil(t, err, "Err %v", err)
	l := clientv3.NewLease(ec.Client)
	respg, err := l.Grant(context.TODO(), 30)
	assert.Nil(t, err)
	log.Printf("resp %x %v\n", respg.ID, respg.TTL)
	respl, err := l.Leases(context.TODO())
	for _, lid := range respl.Leases {
		log.Printf("resp lid %x\n", lid)
	}
	respttl, err := l.TimeToLive(context.TODO(), respg.ID)
	log.Printf("resp %v\n", respttl.TTL)
	ch, err := l.KeepAlive(context.TODO(), respg.ID)
	go func() {
		for respa := range ch {
			log.Printf("respa %v\n", respa.TTL)
		}
	}()
	opts := make([]clientv3.OpOption, 0)
	opts = append(opts, clientv3.WithLease(respg.ID))
	respp, err := ec.Put(context.TODO(), "xxxx", "hello", opts...)
	assert.Nil(t, err)
	log.Printf("put %v\n", respp)
	lopts := make([]clientv3.LeaseOption, 0)
	lopts = append(lopts, clientv3.WithAttachedKeys())
	respttl, err = l.TimeToLive(context.TODO(), respg.ID, lopts...)
	for _, k := range respttl.Keys {
		log.Printf("respttl %v %v\n", respttl.TTL, string(k))
	}
	time.Sleep(60 * time.Second)

	err = l.Close()
	assert.Nil(t, err)
}
