package memfs

// Run go test ulambda/memfs

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

type TestState struct {
	t     *testing.T
	fs    *Root
	rooti *Inode
}

func newTest(t *testing.T) *TestState {
	ts := &TestState{}
	ts.t = t
	ts.fs = MakeRoot()
	ts.rooti = ts.fs.RootInode()
	return ts
}

func TestRoot(t *testing.T) {
	ts := newTest(t)
	assert.Equal(t, ts.fs.inode.Inum, RootInum)
}

func (ts *TestState) initfs() {
	const N = 1000
	_, err := ts.rooti.Create("", ts.fs, np.DMDIR|07000, "todo")
	assert.Nil(ts.t, err, "Create todo")
	is, _, err := ts.rooti.Walk("", []string{"todo"})
	assert.Nil(ts.t, err, "Walk todo")
	for i := 0; i < N; i++ {
		_, err = is[1].Create("", ts.fs, 07000, "job"+strconv.Itoa(i))
		assert.Nil(ts.t, err, "Create job")
	}
}

func (ts *TestState) testRename(t string) {
	done := false
	for !done {
		is, _, err := ts.rooti.Walk(t, []string{"todo"})
		assert.Nil(ts.t, err, "Walk todo")
		assert.Equal(ts.t, len(is), 2, "Walked too few inodes")
		b, err := is[1].Read(0, 100)
		if len(b) == 0 { // are we done?
			done = true
		} else {
			assert.Nil(ts.t, err, "Read todo")
			var st np.Stat
			err = npcodec.Unmarshal(b, &st)
			assert.Nil(ts.t, err, "Unmarshal todo")
			name := st.Name
			if strings.HasPrefix(name, "job") {
				err = ts.fs.Rename(t, np.Split("todo/"+name),
					np.Split("todo/done-"+name))
				if err != nil {
					assert.Contains(ts.t, err.Error(), "Unknown name")
				}
			} else {
				assert.Equal(ts.t, err, nil, "Rename todo")
				_ = ts.rooti.Remove(t, ts.fs, np.Split("todo/"+name))
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
			ts.testRename(strconv.Itoa(t))
			wg.Done()
		}(i)
	}
	wg.Wait()
}
