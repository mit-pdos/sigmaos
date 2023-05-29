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
	crash := 1
	crashinterval := 200
	return groupmgr.Start(sc, 1, "namedv1", []string{strconv.Itoa(crash)}, job, 0, crash, crashinterval, 0, 0)
}

func TestNamedWalk(t *testing.T) {
	ts := makeTstate(t)

	pn := sp.NAMEDV1 + "/"

	ts.waitNamed(t)

	d := []byte("hello")
	_, err := ts.PutFile(path.Join(pn, "f"), 0777, sp.OWRITE, d)
	assert.Nil(t, err)

	ndg := startNamed(ts.SigmaClnt, ts.job)

	// wait until kernel-started named exited and its lease expired
	time.Sleep((etcclnt.SessionTTL + 1) * time.Second)
	ts.waitNamed(t)

	start := time.Now()
	for time.Since(start) < 10*time.Second {
		d1, err := ts.GetFile(path.Join(pn, "f"))
		if err != nil {
			log.Printf("err %v\n", err)
			assert.Nil(t, err)
			break
		}
		assert.Equal(t, d, d1)
	}

	log.Printf("remove f\n")

	mnt1, err := ts.ReadMount(sp.NAMEDV1)
	log.Printf("read mount err %v %v\n", err, mnt1)

	for {
		err := ts.Remove(path.Join(pn, "f"))
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
