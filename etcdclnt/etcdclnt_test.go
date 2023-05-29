package etcdclnt

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/stretchr/testify/assert"

	"sigmaos/groupmgr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestEtcdLs(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	resp, err := cli.Get(context.TODO(), "\000", clientv3.WithRange("\000"), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	assert.Nil(t, err)
	log.Printf("resp %v\n", resp)
	for _, ev := range resp.Kvs {
		fmt.Printf("%s : %s\n", ev.Key, ev.Value)
	}
}

func TestEtcdDelAll(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	resp, err := cli.Delete(context.TODO(), "\000", clientv3.WithRange("\000"))
	assert.Nil(t, err)
	log.Printf("resp %v\n", resp)
}

func leader(ch chan struct{}, i int) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		log.Fatalf("new %v\n", err)
	}
	defer cli.Close()

	var s *concurrency.Session
	s, err = concurrency.NewSession(cli, concurrency.WithTTL(SessionTTL))
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	e := concurrency.NewElection(s, "/leader-election/")

	log.Printf("Campaign %v\n", i)

	err = e.Campaign(ctx, strconv.Itoa(i))
	if err != nil {
		log.Fatalf("campaign %v\n", err)
	}
	log.Printf("%d: campaign %v\n", i, err)

	var leader *clientv3.GetResponse
	leader, err = e.Leader(ctx)
	if err != nil {
		log.Fatalf("Leader() returned non nil err: %s", err)
	}

	log.Printf("Leader %d:%v\n", i, leader)

	time.Sleep((SessionTTL + 1) * time.Second)

	ch <- struct{}{}

	log.Printf("Leader %d:%v done\n", i, leader)
}

func TestEtcdLeader(t *testing.T) {
	const N = 2

	ch := make(chan struct{})
	for i := 0; i < N; i++ {
		go leader(ch, i)
	}

	for i := 0; i < N; i++ {
		<-ch
	}

}

func waitNamed(ts *test.Tstate, t *testing.T) {
	cont := true
	for cont {
		sts, err := ts.GetDir(sp.NAMED)
		assert.Nil(t, err)
		for _, st := range sts {
			if st.Name == "namedv1" {
				log.Printf("namedv1 %v\n", st)
				cont = false
			}
		}
		if cont {
			time.Sleep(1 * time.Second)
		}
	}
	mnt1, err := ts.ReadMount(sp.NAMEDV1)
	log.Printf("read mount err %v %v\n", err, mnt1)
}

func startNamed(sc *sigmaclnt.SigmaClnt, job string) *groupmgr.GroupMgr {
	crash := 1
	crashinterval := 0
	return groupmgr.Start(sc, 1, "namedv1", []string{strconv.Itoa(crash)}, job, 0, crash, crashinterval, 0, 0)
}

func TestGetNamed(t *testing.T) {
	ts := test.MakeTstateAll(t)

	ndg := startNamed(ts.SigmaClnt, "xxx")

	// wait until kernel-started named exited and its lease expired
	time.Sleep((SessionTTL + 1) * time.Second)

	waitNamed(ts, t)
	err := GetNamed()
	assert.Nil(t, err)

	ndg.Stop()

	ts.Shutdown()
}
