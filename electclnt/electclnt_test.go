package electclnt_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/electclnt"
	"sigmaos/test"
)

const (
	LEADERNAME = "name/leader"
)

func TestAcquireRelease(t *testing.T) {
	ts := test.NewTstate(t)

	N := 20

	leader1, err := electclnt.NewElectClnt(ts.FsLib, LEADERNAME, 0)
	assert.Nil(t, err)
	leader2, err := electclnt.NewElectClnt(ts.FsLib, LEADERNAME, 0)
	assert.Nil(t, err)

	for i := 0; i < N; i++ {
		err := leader1.AcquireLeadership([]byte{})
		assert.Nil(ts.T, err, "AcquireLeadership")
		err = leader1.ReleaseLeadership()
		assert.Nil(ts.T, err, "ReleaseLeadership")
		err = leader2.AcquireLeadership([]byte{})
		assert.Nil(ts.T, err, "AcquireLeadership")
		err = leader2.ReleaseLeadership()
		assert.Nil(ts.T, err, "ReleaseLeadership")
	}

	ts.Shutdown()
}

// n thread become try to become a leader and on success add 1 to shared file
func TestLeaderConcur(t *testing.T) {
	ts := test.NewTstate(t)

	N := 3000
	n_threads := 20
	cnt := 0

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, N *int, cnt *int) {
			defer done.Done()
			for {
				leader, err := electclnt.NewElectClnt(ts.FsLib, LEADERNAME, 0)
				assert.Nil(t, err)
				err = leader.AcquireLeadership([]byte{})
				assert.Nil(ts.T, err, "AcquireLeader")
				if *cnt < *N {
					*cnt += 1
				} else {
					err = leader.ReleaseLeadership()
					assert.Nil(ts.T, err, "ReleaseLeadership")
					break
				}
				err = leader.ReleaseLeadership()
				assert.Nil(ts.T, err, "ReleaseLeadership")
			}
		}(&done, &N, &cnt)
	}

	done.Wait()
	assert.Equal(ts.T, N, cnt, "Count doesn't match up")

	ts.Shutdown()
}

// n thread become leader in turn and add 1
func TestLeaderInTurn(t *testing.T) {
	ts := test.NewTstate(t)

	N := uint64(20)
	sum := uint64(0)
	current := uint64(0)
	done := make(chan uint64)

	for i := uint64(0); i < N; i++ {
		go func(i uint64) {
			leader, err := electclnt.NewElectClnt(ts.FsLib, LEADERNAME, 0)
			assert.Nil(t, err)
			me := false
			for !me {
				err := leader.AcquireLeadership([]byte{})
				assert.Nil(ts.T, err, "AcquireLeadership")
				if atomic.LoadUint64(&current) == i {
					me = true
				}
				err = leader.ReleaseLeadership()
				assert.Nil(ts.T, err, "ReleaseLeadership")
				if me {
					atomic.AddUint64(&current, 1)
					done <- i
				}
			}
		}(i)
		sum += i
	}

	for i := uint64(0); i < N; i++ {
		next := <-done
		assert.Equal(ts.T, i, next, "Next (%v) not equal to expected (%v)", next, i)
	}

	ts.Shutdown()
}
