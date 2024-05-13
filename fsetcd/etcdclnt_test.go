package fsetcd_test

import (
	"context"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
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
	db.DPrintf(db.TEST, "resp %x %v\n", respg.ID, respg.TTL)
	respl, err := l.Leases(context.TODO())
	for _, lid := range respl.Leases {
		db.DPrintf(db.TEST, "resp lid %x\n", lid)
	}
	respttl, err := l.TimeToLive(context.TODO(), respg.ID)
	db.DPrintf(db.TEST, "resp %v\n", respttl.TTL)
	ch, err := l.KeepAlive(context.TODO(), respg.ID)
	go func() {
		for respa := range ch {
			db.DPrintf(db.TEST, "respa %v\n", respa.TTL)
		}
	}()
	opts := make([]clientv3.OpOption, 0)
	opts = append(opts, clientv3.WithLease(respg.ID))
	respp, err := ec.Put(context.TODO(), "xxxx", "hello", opts...)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "put %v\n", respp)
	lopts := make([]clientv3.LeaseOption, 0)
	lopts = append(lopts, clientv3.WithAttachedKeys())
	respttl, err = l.TimeToLive(context.TODO(), respg.ID, lopts...)
	for _, k := range respttl.Keys {
		db.DPrintf(db.TEST, "respttl %v %v\n", respttl.TTL, string(k))
	}
	time.Sleep(60 * time.Second)

	err = l.Close()
	assert.Nil(t, err)
}

func TestEvents(t *testing.T) {
	lip := sp.Tip("127.0.0.1")
	_, _, amgr, err := test.NewAuthMgr()
	assert.Nil(t, err)
	secrets := map[string]*proc.ProcSecretProto{}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(amgr, sp.Tip(test.EtcdIP))
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, lip, lip, "", false, false, false, false)
	npc := netproxyclnt.NewNetProxyClnt(pe, nil)
	ec, err := fsetcd.NewFsEtcd(npc.Dial, pe.GetEtcdEndpoints(), pe.GetRealm())
	assert.Nil(t, err, "Err %v", err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wopts := make([]clientv3.OpOption, 0)
	// wopts = append(wopts, clientv3.WithRev(1))
	wopts = append(wopts, clientv3.WithPrefix())
	wopts = append(wopts, clientv3.WithFilterPut())
	wCtx, wCancel := context.WithCancel(ctx)
	wch := ec.Watch(wCtx, "x", wopts...)
	assert.NotNil(t, wch)

	go func() error {
		for {
			watchResp, ok := <-wch
			if ok {
				db.DPrintf(db.TEST, "watchResp %v\n", watchResp)
				for _, kv := range watchResp.Events {
					db.DPrintf(db.TEST, "watchResp event %v\n", kv)
				}

			} else {
				db.DPrintf(db.TEST, "wch closed\n")
				return nil
			}
		}
	}()

	opts := make([]clientv3.OpOption, 0)
	respp, err := ec.Put(context.TODO(), "xx", "hello", opts...)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "put %v\n", respp)
	respd, err := ec.Delete(context.TODO(), "xx", opts...)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "delete %v\n", respd)

	time.Sleep(1 * time.Second)

	wCancel()
}
