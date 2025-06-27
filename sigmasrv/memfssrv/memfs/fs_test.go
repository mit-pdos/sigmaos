package memfs

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
)

type TestState struct {
	t     *testing.T
	rooti fs.Dir
	ctx   fs.CtxI
}

func newTest(t *testing.T) *TestState {
	ts := &TestState{}
	ts.t = t
	ts.ctx = ctx.NewCtxNull()
	ts.rooti = dir.NewRootDir(ts.ctx, memfs.NewNewInode(sp.DEV_MEMFS))
	return ts
}

func (ts *TestState) initfs() {
	const N = 1000
	_, err := ts.rooti.Create(ts.ctx, "done", sp.DMDIR|07000, 0, sp.NoLeaseId, sp.NoFence(), nil)
	assert.Nil(ts.t, err, "Create done")
	_, err = ts.rooti.Create(ts.ctx, "todo", sp.DMDIR|07000, 0, sp.NoLeaseId, sp.NoFence(), nil)
	assert.Nil(ts.t, err, "Create todo")
	_, _, _, err = ts.rooti.LookupPath(ts.ctx, path.Tpathname{"todo"})
	assert.Nil(ts.t, err, "Walk todo")
}

func (ts *TestState) testRename(t int) {
	_, lo, _, err := ts.rooti.LookupPath(ts.ctx, path.Tpathname{"todo"})
	assert.Nil(ts.t, err, "Lookup todo")
	d1 := lo.(fs.Dir)

	_, lo, _, err = ts.rooti.LookupPath(ts.ctx, path.Tpathname{"done"})
	assert.Nil(ts.t, err, "Lookup done")
	d2 := lo.(fs.Dir)

	sts, err := d1.ReadDir(ts.ctx, 0, 100)
	assert.Nil(ts.t, err, "ReadDir")
	for _, st := range sts {
		_, _, _, err := d1.LookupPath(ts.ctx, path.Tpathname{st.Name})
		if err != nil {
			continue
		}
		err = d1.Renameat(ts.ctx, st.Name, d2, st.Name, sp.NoFence())
		if err != nil {
			assert.True(ts.t, serr.IsErrCode(err, serr.TErrNotfound))
		}
	}
}

func TestSimple(t *testing.T) {
	ts := newTest(t)
	ts.initfs()
}

func TestConcurRename(t *testing.T) {
	const N = 10
	ts := newTest(t)
	ts.initfs()

	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(t int) {
			ts.testRename(t)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
