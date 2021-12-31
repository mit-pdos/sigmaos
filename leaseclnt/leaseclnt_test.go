package leaseclnt_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/leaseclnt"
	np "ulambda/ninep"
)

const (
	LEASE_DIR = "name/lease"
	LEASENAME = "name/test-lease"
)

type Tstate struct {
	t *testing.T
	*kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemNamed("sync_test", "..")
	ts.Mkdir(np.LOCKS, 0777)
	return ts
}

func TestLease1(t *testing.T) {
	ts := makeTstate(t)

	N := 20
	sum := 0
	current := 0
	done := make(chan int)

	lease := leaseclnt.MakeLeaseClnt(ts.FsLib, LEASENAME, 0)

	for i := 0; i < N; i++ {
		go func(i int) {
			me := false
			for !me {
				err := lease.WaitWLease([]byte{})
				assert.Nil(ts.t, err, "WaitWLease")
				if current == i {
					current += 1
					done <- i
					me = true
				}
				err = lease.ReleaseWLease()
				assert.Nil(ts.t, err, "ReleaseWLease")
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

func TestLease2(t *testing.T) {
	ts := makeTstate(t)

	N := 20

	lease1 := leaseclnt.MakeLeaseClnt(ts.FsLib, LEASENAME+"-1234", 0)
	lease2 := leaseclnt.MakeLeaseClnt(ts.FsLib, LEASENAME+"-1234", 0)

	for i := 0; i < N; i++ {
		err := lease1.WaitWLease([]byte{})
		assert.Nil(ts.t, err, "WaitWLease")
		err = lease1.ReleaseWLease()
		assert.Nil(ts.t, err, "ReleaseWLease")
		err = lease2.WaitWLease([]byte{})
		assert.Nil(ts.t, err, "WaitWLease")
		err = lease2.ReleaseWLease()
		assert.Nil(ts.t, err, "ReleaseWLease")
	}

	ts.Shutdown()
}

func TestLease3(t *testing.T) {
	ts := makeTstate(t)

	N := 3000
	n_threads := 20
	cnt := 0

	lease := leaseclnt.MakeLeaseClnt(ts.FsLib, LEASENAME+"-1234", 0)

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, lease *leaseclnt.LeaseClnt, N *int, cnt *int) {
			defer done.Done()
			for {
				err := lease.WaitWLease([]byte{})
				assert.Nil(ts.t, err, "WaitWLease")
				if *cnt < *N {
					*cnt += 1
				} else {
					err = lease.ReleaseWLease()
					assert.Nil(ts.t, err, "ReleaseWLease")
					break
				}
				err = lease.ReleaseWLease()
				assert.Nil(ts.t, err, "ReleaseWLease")
			}
		}(&done, lease, &N, &cnt)
	}

	done.Wait()
	assert.Equal(ts.t, N, cnt, "Count doesn't match up")

	ts.Shutdown()
}

// Test if an exit of another session doesn't remove ephemeral files
// of another session.
func TestLease4(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LEASE_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	fsl1 := fslib.MakeFsLibAddr("fslib-1", fslib.Named())
	fsl2 := fslib.MakeFsLibAddr("fslib-2", fslib.Named())

	lease1 := leaseclnt.MakeLeaseClnt(fsl1, LEASENAME, 0)

	// Establish a connection
	_, err = fsl2.ReadDir(LEASE_DIR)
	assert.Nil(ts.t, err, "ReadDir")

	err = lease1.WaitWLease([]byte{})
	assert.Nil(ts.t, err, "WaitWLease")

	fsl2.Exit()

	time.Sleep(2 * time.Second)

	err = lease1.ReleaseWLease()
	assert.Nil(ts.t, err, "ReleaseWLease")
	ts.Shutdown()
}
