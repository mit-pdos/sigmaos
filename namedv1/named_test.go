package namedv1

import (
	"context"
	"fmt"
	"log"
	"path"
	"testing"

	"github.com/coreos/etcd/clientv3"

	"github.com/stretchr/testify/assert"

	spath "sigmaos/path"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestKey(t *testing.T) {
	log.Printf("key %v\n", path2key(spath.Path{}))
	log.Printf("key %v\n", path2key(spath.Path{"f"}))
	log.Printf("key %v\n", path2key(spath.Path{"d", "f"}))
}

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

func TestEtcdDir(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	pns := []string{"a", "a/b", "a/c", "a/d/a", "a/e", "a/e/a"}
	assert.Nil(t, err)
	for _, pn := range pns {
		p := spath.Split(pn)
		k := path2key(p)
		resp, err := cli.Put(context.TODO(), k, p.String())
		assert.Nil(t, err)
		log.Printf("put resp %v %v\n", k, resp)
	}
	resp, err := cli.Get(context.TODO(), "1:a", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	assert.Nil(t, err)
	log.Printf("resp %v\n", resp)
	for _, ev := range resp.Kvs {
		fmt.Printf("%s : %s\n", ev.Key, ev.Value)
	}
}

func TestTxn(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})

	resp, err := cli.Put(context.TODO(), "f", "hello")
	assert.Nil(t, err)
	log.Printf("put resp %v\n", resp)
	gresp, err := cli.Get(context.TODO(), "f")
	log.Printf("get resp %v\n", gresp)
	b := gresp.Kvs[0].Value
	v := gresp.Kvs[0].Version
	tresp, err := cli.Txn(context.TODO()).
		If(clientv3.Compare(clientv3.Version("f"), "=", v)).Then(clientv3.OpPut("g", string(b)), clientv3.OpDelete("f")).Commit()
	assert.Nil(t, err)
	log.Printf("txn resp %v\n", tresp)
}

func TestOne(t *testing.T) {
	ts := test.MakeTstateAll(t)

	pn := sp.NAMEDV1 + "/"

	d := []byte("hello")
	_, err := ts.PutFile(path.Join(pn, "f"), 0777, sp.OWRITE, d)
	assert.Nil(t, err)

	d1, err := ts.GetFile(path.Join(pn, "f"))
	assert.Nil(t, err)
	assert.Equal(t, d, d1)

	sts, err := ts.GetDir(pn)
	assert.Nil(t, err, "GetDir")

	log.Printf("%v dirents %v\n", sp.NAMEDV1, sts)
	assert.Equal(t, 3, len(sts))

	err = ts.Remove(path.Join(pn, "f"))
	assert.Nil(t, err)

	sts, err = ts.GetDir(pn)
	assert.Nil(t, err, "GetDir")

	log.Printf("%v dirents %v\n", sp.NAMEDV1, sts)
	assert.Equal(t, 2, len(sts))

	ts.Shutdown()
}
