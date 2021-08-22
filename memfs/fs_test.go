package memfs

// Run go test ulambda/memfs

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fs"
	np "ulambda/ninep"
)

type Ctx struct {
	uname string
}

func MkCtx(uname string) *Ctx {
	return &Ctx{uname}
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}

type TestState struct {
	t     *testing.T
	rooti *Dir
	ctx   fs.CtxI
}

func newTest(t *testing.T) *TestState {
	ts := &TestState{}
	ts.t = t
	ts.rooti = MkRootInode()
	ts.ctx = MkCtx("")
	return ts
}

func (ts *TestState) initfs() {
	const N = 1000
	_, err := ts.rooti.Create(ts.ctx, "done", np.DMDIR|07000, 0)
	assert.Nil(ts.t, err, "Create done")
	_, err = ts.rooti.Create(ts.ctx, "todo", np.DMDIR|07000, 0)
	assert.Nil(ts.t, err, "Create todo")
	is, _, err := ts.rooti.Lookup(ts.ctx, []string{"todo"})
	assert.Nil(ts.t, err, "Walk todo")
	for i := 0; i < N; i++ {
		_, err = is[0].(*Dir).Create(ts.ctx, "job"+strconv.Itoa(i), 07000, 0)
		assert.Nil(ts.t, err, "Create job")
	}
}

func (ts *TestState) testRename(t int) {
	is, _, err := ts.rooti.Lookup(ts.ctx, []string{"todo"})
	assert.Nil(ts.t, err, "Lookup todo")
	assert.Equal(ts.t, 1, len(is), "Walked too few inodes")
	d1 := is[0].(*Dir)

	is, _, err = ts.rooti.Lookup(ts.ctx, []string{"done"})
	assert.Nil(ts.t, err, "Lookup done")
	assert.Equal(ts.t, 1, len(is), "Walked too few inodes")
	d2 := is[0].(*Dir)

	sts, err := d1.ReadDir(ts.ctx, 0, 100, np.NoV)
	assert.Nil(ts.t, err, "ReadDir")
	for _, st := range sts {
		is, _, err := d1.Lookup(ts.ctx, []string{st.Name})
		if len(is) == 0 {
			continue
		}
		err = d1.Renameat(ts.ctx, st.Name, d2, st.Name)
		if err != nil {
			assert.Contains(ts.t, err.Error(), "file not found")
		}
	}
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
