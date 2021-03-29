package memfs

// Run go test ulambda/memfs

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

type TestState struct {
	t     *testing.T
	rooti *Inode
	ctx   npo.CtxI
}

func newTest(t *testing.T) *TestState {
	ts := &TestState{}
	ts.t = t
	ts.rooti = MkRootInode()
	ts.ctx = DefMkCtx("")
	return ts
}

func (ts *TestState) initfs() {
	const N = 1000
	_, err := ts.rooti.Create(ts.ctx, "todo", np.DMDIR|07000, 0)
	assert.Nil(ts.t, err, "Create todo")
	is, _, err := ts.rooti.Lookup(ts.ctx, []string{"todo"})
	assert.Nil(ts.t, err, "Walk todo")
	for i := 0; i < N; i++ {
		_, err = is[0].Create(ts.ctx, "job"+strconv.Itoa(i), 07000, 0)
		assert.Nil(ts.t, err, "Create job")
	}
}

func (ts *TestState) testRename(t int) {
	is, _, err := ts.rooti.Lookup(ts.ctx, []string{"todo"})
	assert.Nil(ts.t, err, "Lookup todo")
	assert.Equal(ts.t, 1, len(is), "Walked too few inodes")
	ino := is[0].(*Inode)
	sts, err := ino.ReadDir(ts.ctx, 0, 100, np.NoV)
	assert.Nil(ts.t, err, "ReadDir")
	for _, st := range sts {
		is, _, err := ino.Lookup(ts.ctx, []string{st.Name})
		if len(is) == 0 {
			continue
		}
		assert.Nil(ts.t, err, "Lookup name "+st.Name)
		assert.Equal(ts.t, 1, len(is), "Walked too few inodes")
		ino2 := is[0].(*Inode)
		if strings.HasPrefix(st.Name, "job") {
			err = ino2.Rename(ts.ctx, st.Name, "done-"+st.Name)
			if err != nil {
				assert.Contains(ts.t, err.Error(), "file not found")
			}
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
