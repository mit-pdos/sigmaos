package namedv1

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

	"sigmaos/groupmgr"
	rd "sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestEtcdLs(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
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
		DialTimeout: dialTimeout,
	})
	resp, err := cli.Delete(context.TODO(), "\000", clientv3.WithRange("\000"))
	assert.Nil(t, err)
	log.Printf("resp %v\n", resp)
}

func TestEtcdLeader(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	defer cli.Close()
	var s *concurrency.Session
	s, err = concurrency.NewSession(cli)
	assert.Nil(t, err)
	defer s.Close()

	assert.Nil(t, err)
	e := concurrency.NewElection(s, "/leader-election/")
	err = e.Campaign(context.Background(), "0")
	assert.Nil(t, err)
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

func startNamed(sc *sigmaclnt.SigmaClnt, job string) *groupmgr.GroupMgr {
	return groupmgr.Start(sc, 1, "namedv1", []string{strconv.Itoa(0)}, job, 0, 1, 0, 0, 0)
}

func TestNamedLeader(t *testing.T) {
	ts := makeTstate(t)

	time.Sleep(5 * time.Second)

	startNamed(ts.SigmaClnt, ts.job)

	pn := sp.NAMEDV1 + "/"

	for i := 0; i < 30; i++ {
		log.Printf("%d\n", i)
		d := []byte("iter-" + strconv.Itoa(i))
		_, err := ts.PutFile(path.Join(pn, "f"), 0777, sp.OWRITE, d)
		assert.Nil(t, err)

		d1, err := ts.GetFile(path.Join(pn, "f"))
		assert.Nil(t, err)
		assert.Equal(t, d, d1)

		time.Sleep(1)
	}

	err := ts.Remove(path.Join(pn, "f"))
	assert.Nil(t, err)

	// ndg.Wait()

	ts.Shutdown()
}
