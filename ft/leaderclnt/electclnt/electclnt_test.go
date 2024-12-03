package electclnt_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	"sigmaos/ft/leaderclnt/electclnt"
	"sigmaos/test"
)

const (
	LEADERNAME = "name/leader"
)

func TestCompile(t *testing.T) {
}

func TestAcquireRelease(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

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

// n thread become try to become a leader and on success keep adding
// a shared counter until N
func TestLeaderConcur(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	N := 3000
	ntreads := 20
	cnt := 0

	var done sync.WaitGroup
	done.Add(ntreads)

	for i := 0; i < ntreads; i++ {
		go func(done *sync.WaitGroup, N *int, cnt *int, i int) {
			defer done.Done()
			for {
				leader, err := electclnt.NewElectClnt(ts.FsLib, LEADERNAME, 0)
				assert.Nil(t, err)
				err = leader.AcquireLeadership([]byte{})
				assert.Nil(ts.T, err, "AcquireLeader")
				// db.DPrintf(db.TEST, " %d leader", i)
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
		}(&done, &N, &cnt, i)
	}

	done.Wait()
	assert.Equal(ts.T, N, cnt, "Count doesn't match up")

	ts.Shutdown()
}

// n thread become leader in turn and add 1
func TestLeaderInTurn(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	N := uint64(20)
	sum := uint64(0)
	var current atomic.Uint64
	done := make(chan uint64)

	for i := uint64(0); i < N; i++ {
		go func(i uint64) {
			leader, err := electclnt.NewElectClnt(ts.FsLib, LEADERNAME, 0)
			assert.Nil(t, err)
			me := false
			for !me {
				err := leader.AcquireLeadership([]byte{})
				assert.Nil(ts.T, err, "AcquireLeadership")
				if current.Load() == i {
					me = true
				}
				err = leader.ReleaseLeadership()
				assert.Nil(ts.T, err, "ReleaseLeadership")
				if me {
					current.Add(1)
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
