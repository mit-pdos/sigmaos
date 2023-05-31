package etcdclnt_test

import (
	"context"
	"fmt"
	"log"
	"path"
	"strconv"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/stretchr/testify/assert"

	"sigmaos/etcdclnt"
	"sigmaos/groupmgr"
	rd "sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestEtcdLs(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdclnt.Endpoints,
		DialTimeout: etcdclnt.DialTimeout,
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
		Endpoints:   etcdclnt.Endpoints,
		DialTimeout: etcdclnt.DialTimeout,
	})
	resp, err := cli.Delete(context.TODO(), "\000", clientv3.WithRange("\000"))
	assert.Nil(t, err)
	log.Printf("resp %v\n", resp)
}

func leader(ch chan struct{}, i int) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdclnt.Endpoints,
		DialTimeout: etcdclnt.DialTimeout,
	})
	if err != nil {
		log.Fatalf("new %v\n", err)
	}
	defer cli.Close()

	var s *concurrency.Session
	s, err = concurrency.NewSession(cli, concurrency.WithTTL(etcdclnt.SessionTTL))
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

	time.Sleep((etcdclnt.SessionTTL + 1) * time.Second)

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

func startNamed(sc *sigmaclnt.SigmaClnt, job string, crash, crashinterval int) *groupmgr.GroupMgr {
	return groupmgr.Start(sc, 1, "namedv1", []string{strconv.Itoa(crash)}, job, 0, crash, crashinterval, 0, 0)
}

func TestBootNamed(t *testing.T) {
	crash := 1
	crashinterval := 0

	ts := test.MakeTstateAll(t)

	ndg := startNamed(ts.SigmaClnt, "xxx", crash, crashinterval)

	// wait until kernel-started named exited and its lease expired
	time.Sleep((etcdclnt.SessionTTL + 3) * time.Second)

	sts, err1 := ts.GetDir(sp.NAMEDV1 + "/")
	assert.Nil(t, err1)
	log.Printf("named %v\n", sp.Names(sts))

	ndg.Stop()

	ts.Shutdown()
}

type Tstate struct {
	job string
	*test.Tstate
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.job = rd.String(4)
	return ts
}

func TestNamedWalk(t *testing.T) {
	crash := 1
	crashinterval := 200
	// crashinterval := 0

	ts := makeTstate(t)

	pn := sp.NAMEDV1 + "/"

	d := []byte("hello")
	_, err := ts.PutFile(path.Join(pn, "testf"), 0777, sp.OWRITE, d)
	assert.Nil(t, err)

	ndg := startNamed(ts.SigmaClnt, ts.job, crash, crashinterval)

	// wait until kernel-started named exited and its lease expired
	time.Sleep((etcdclnt.SessionTTL + 2) * time.Second)

	start := time.Now()
	i := 0
	for time.Since(start) < 10*time.Second {
		d1, err := ts.GetFile(path.Join(pn, "testf"))
		if err != nil {
			log.Printf("err %v\n", err)
			assert.Nil(t, err)
			break
		}
		assert.Equal(t, d, d1)
		i += 1
	}

	log.Printf("#getfile %d\n", i)

	for {
		err := ts.Remove(path.Join(pn, "testf"))
		if err == nil {
			break
		}
		log.Printf("remove f retry\n")
		time.Sleep(100 * time.Millisecond)
	}

	ndg.Stop()

	log.Printf("namedv1 stopped\n")

	ts.Shutdown()
}
