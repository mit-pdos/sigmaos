package fenceclnt_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fenceclnt"
	"ulambda/fslib"
	"ulambda/kernel"
)

const (
	FENCENAME = "name/test-fence"
)

type Tstate struct {
	t *testing.T
	*kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemNamed("fenceclnt_test", "..")
	ts.Mkdir(fenceclnt.FENCE_DIR, 0777)
	return ts
}

func TestFence1(t *testing.T) {
	ts := makeTstate(t)

	N := 20
	sum := 0
	current := 0
	done := make(chan int)

	fence := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME, 0)

	for i := 0; i < N; i++ {
		go func(i int) {
			me := false
			for !me {
				err := fence.AcquireFenceW([]byte{})
				assert.Nil(ts.t, err, "AcquieFenceW")
				if current == i {
					current += 1
					done <- i
					me = true
				}
				err = fence.ReleaseFence()
				assert.Nil(ts.t, err, "ReleaseFence")
			}
		}(i)
		sum += i
	}

	for i := 0; i < N; i++ {
		next := <-done
		assert.Equal(ts.t, i, next, "Next (%v) not equal to expected (%v)", next, i)
	}

	ts.Shutdown()
}

func TestFence2(t *testing.T) {
	ts := makeTstate(t)

	N := 20

	fence1 := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME+"-1234", 0)
	fence2 := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME+"-1234", 0)

	for i := 0; i < N; i++ {
		err := fence1.AcquireFenceW([]byte{})
		assert.Nil(ts.t, err, "AcquireFenceW")
		err = fence1.ReleaseFence()
		assert.Nil(ts.t, err, "ReleaseFence")
		err = fence2.AcquireFenceW([]byte{})
		assert.Nil(ts.t, err, "AcquireFenceW")
		err = fence2.ReleaseFence()
		assert.Nil(ts.t, err, "ReleaseFence")
	}

	ts.Shutdown()
}

func TestLease3(t *testing.T) {
	ts := makeTstate(t)

	N := 3000
	n_threads := 20
	cnt := 0

	fence := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME+"-1234", 0)

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, fence *fenceclnt.FenceClnt, N *int, cnt *int) {
			defer done.Done()
			for {
				err := fence.AcquireFenceW([]byte{})
				assert.Nil(ts.t, err, "AcquireFence")
				if *cnt < *N {
					*cnt += 1
				} else {
					err = fence.ReleaseFence()
					assert.Nil(ts.t, err, "ReleaseFence")
					break
				}
				err = fence.ReleaseFence()
				assert.Nil(ts.t, err, "ReleaseFence")
			}
		}(&done, fence, &N, &cnt)
	}

	done.Wait()
	assert.Equal(ts.t, N, cnt, "Count doesn't match up")

	ts.Shutdown()
}

// Test if an exit of another session doesn't remove ephemeral files
// of another session.
func TestFence4(t *testing.T) {
	ts := makeTstate(t)

	fsl1 := fslib.MakeFsLibAddr("fslib-1", fslib.Named())
	fsl2 := fslib.MakeFsLibAddr("fslib-2", fslib.Named())

	fence1 := fenceclnt.MakeFenceClnt(fsl1, FENCENAME, 0)

	// Establish a connection
	_, err := fsl2.ReadDir(fenceclnt.FENCE_DIR)
	assert.Nil(ts.t, err, "ReadDir")

	err = fence1.AcquireFenceW([]byte{})
	assert.Nil(ts.t, err, "AcquireFenceW")

	fsl2.Exit()

	time.Sleep(2 * time.Second)

	err = fence1.ReleaseFence()
	assert.Nil(ts.t, err, "ReleaseFence")
	ts.Shutdown()
}
