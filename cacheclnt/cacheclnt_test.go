package cacheclnt_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	"sigmaos/groupmgr"
	rd "sigmaos/rand"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	cm      *cacheclnt.CacheMgr
	grpmgrs []*groupmgr.GroupMgr
}

func mkTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.cm = cacheclnt.MkCacheMgr(ts.FsLib, ts.ProcClnt, rd.String(8), 2)
	ts.cm.StartCache()
	return ts
}

func TestCache(t *testing.T) {
	const (
		N      = 10
		NSHARD = 2
	)
	ts := mkTstate(t)
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib)
	assert.Nil(t, err)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err = cc.Set(key, key)
		assert.Nil(t, err)
	}

	for g := 0; g < cacheclnt.NCACHE; g++ {
		m, err := cc.Dump(g)
		assert.Nil(t, err)
		assert.Equal(t, 5, len(m))
	}

	ts.cm.StopCache()
	ts.Shutdown()
}
