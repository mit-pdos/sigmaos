package cacheclnt_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	"sigmaos/cachesrv"
	"sigmaos/group"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	job     string
	grpmgrs []*groupmgr.GroupMgr
}

func mkTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.job = rd.String(8)
	ts.Tstate = test.MakeTstateAll(t)
	return ts
}

func (ts *Tstate) startCache(n int) {
	for g := 0; g < n; g++ {
		gn := group.GRP + strconv.Itoa(g)
		grpmgr := groupmgr.Start(ts.FsLib, ts.ProcClnt, 1, "user/hotel-cached", []string{gn}, ts.job, proc.Tcore(1), 0, 0, 0, 0)
		ts.grpmgrs = append(ts.grpmgrs, grpmgr)
	}
}

func (ts *Tstate) stopCache() {
	for _, grpmgr := range ts.grpmgrs {
		err := grpmgr.Stop()
		assert.Nil(ts.T, err)
	}
}

func (ts *Tstate) stop() {
	ts.stopCache()
}

func TestShardedCache(t *testing.T) {
	const (
		N      = 10
		NSHARD = 2
	)

	ts := mkTstate(t)

	ts.startCache(NSHARD)

	cc, err := cacheclnt.MkCacheClnt(ts.FsLib)
	assert.Nil(t, err)

	arg := cachesrv.CacheRequest{}
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		arg.Key = key
		arg.Value = []byte(key)
		res := &cachesrv.CacheResult{}
		err = cc.RPC("Cache.Set", arg, &res)
		assert.Nil(t, err)
	}

	for g := 0; g < cacheclnt.NCACHE; g++ {
		m, err := cc.Dump(g)
		assert.Nil(t, err)
		assert.Equal(t, 5, len(m))
	}

	ts.stop()
	ts.Shutdown()
}
