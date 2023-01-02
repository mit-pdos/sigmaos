package shardsvcmgr_test

import (
	"log"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	"sigmaos/fslib"
	"sigmaos/mon"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/test"
)

const (
	SRV = "user/cached"
)

type Tstate struct {
	*test.Tstate
	job string
	pid proc.Tpid
	cm  *cacheclnt.CacheMgr
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.job = rd.String(8)
	cm, err := cacheclnt.MkCacheMgr(ts.FsLib, ts.ProcClnt, ts.job, 1)
	assert.Nil(t, err)
	ts.cm = cm
	return ts
}

func (ts *Tstate) stop() {
	ts.cm.Stop()
}

func (ts *Tstate) hit(t int) {
	s := strconv.Itoa(t)
	fsl, err := fslib.MakeFsLibAddr("montest"+s, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(t, err)
	cc, err := cacheclnt.MkCacheClnt(fsl, 1)
	assert.Nil(ts.T, err)
	for i := 0; i < 100; i++ {
		cc.Set(s+strconv.Itoa(i), strconv.Itoa(i))
	}
}

func TestCache(t *testing.T) {
	ts := makeTstate(t)
	srv := ts.cm.Server(0)
	log.Printf("srv %s\n", srv)
	ts.hit(0)
	err := mon.Run([]string{"", srv, SRV})
	assert.Nil(t, err)
	ts.stop()
}
