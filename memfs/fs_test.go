package memfs

// Run go test ulambda/memfs

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	fmt.Printf("TestGetRoot\n")
	ts := newTest(t)
	assert.Equal(t, ts.fs.inode.Inum, RootInum)
}

func (ts *TestState) initfs() {
	_, err := ts.rooti.Create(ts.fs, np.DMDIR|07000, "todo")
	require.Nil(ts.t, err, "Create todo")
	is, _, err := ts.rooti.Walk([]string{"todo"})
	require.Nil(ts.t, err, "Walk todo")
	for i := 0; i < 100; i++ {
		_, err = is[1].Create(ts.fs, 07000, "job"+strconv.Itoa(i))
		require.Nil(ts.t, err, "Create job")
	}
	_, err = ts.rooti.Create(ts.fs, np.DMDIR|07000, "started")
	require.Nil(ts.t, err, "Create started")
	_, err = ts.rooti.Create(ts.fs, np.DMDIR|07000, "reduce")
	require.Nil(ts.t, err, "Create reduce")
}

func split(path string) []string {
	p := strings.Split(path, "/")
	return p
}
func (ts *TestState) testRename(t int) {
	log.Print("run ", t)
	done := false
	for !done {
		is, _, err := ts.rooti.Walk([]string{"todo"})
		require.Nil(ts.t, err, "Walk todo")
		assert.Equal(ts.t, len(is), 2, "Walk too few inodes")
		b, err := is[1].Read(0, 256)
		if len(b) == 0 { // are we done?
			done = true
		} else {
			require.Nil(ts.t, err, "Read todo")
			log.Print("b ", len(b))
			var st np.Stat
			err = npcodec.Unmarshal(b, &st)
			require.Nil(ts.t, err, "Unmarshal todo")
			name := st.Name
			err = ts.fs.Rename(split("todo/"+name), split("started/"+name))
			require.Nil(ts.t, err, "Rename todo ", name)
		}
	}

}

func TestConcurRename(t *testing.T) {
	const N = 2
	ts := newTest(t)
	ts.initfs()

	log.Print("run test")

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
