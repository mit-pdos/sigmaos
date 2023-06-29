package leaderetcd_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	"sigmaos/leaderetcd"
	sp "sigmaos/sigmap"
)

func leader(t *testing.T, ch chan struct{}, i int) {
	cli, err := fsetcd.MkFsEtcd(sp.ROOTREALM)
	if err != nil {
		log.Fatalf("new %v\n", err)
	}
	defer cli.Close()
	elect, err := leaderetcd.MkElection(cli.Client, "leader-election")
	assert.Nil(t, err)
	err = elect.Candidate()
	assert.Nil(t, err)

	l, err := elect.Leader(context.Background())
	assert.Nil(t, err)

	log.Printf("Leader %d:%v\n", i, l)

	time.Sleep((fsetcd.SessionTTL + 1) * time.Second)

	ch <- struct{}{}

	log.Printf("Leader %d:%v done\n", i, l)
}

func TestEtcdLeader(t *testing.T) {
	const N = 2

	ch := make(chan struct{})
	for i := 0; i < N; i++ {
		go leader(t, ch, i)
	}

	for i := 0; i < N; i++ {
		<-ch
	}

}
