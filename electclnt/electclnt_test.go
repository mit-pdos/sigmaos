package electclnt_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/electclnt"
	"ulambda/fslib"
	"ulambda/test"
)

const (
	LEADERNAME = "name/leader"
)

func TestAcquireRelease(t *testing.T) {
	ts := test.MakeTstate(t)

	N := 20

	leader1 := electclnt.MakeElectClnt(ts.FsLib, LEADERNAME, 0)
	leader2 := electclnt.MakeElectClnt(ts.FsLib, LEADERNAME, 0)

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
	ts := test.MakeTstate(t)

	N := 3000
	n_threads := 20
	cnt := 0

	leader := electclnt.MakeElectClnt(ts.FsLib, LEADERNAME, 0)

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, leader *electclnt.ElectClnt, N *int, cnt *int) {
			defer done.Done()
			for {
				err := leader.AcquireLeadership([]byte{})
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
		}(&done, leader, &N, &cnt)
	}

	done.Wait()
	assert.Equal(ts.T, N, cnt, "Count doesn't match up")

	ts.Shutdown()
}

// n thread become leader in turn and add 1
func TestLeaderInTurn(t *testing.T) {
	ts := test.MakeTstate(t)

	N := 20
	sum := 0
	current := 0
	done := make(chan int)

	leader := electclnt.MakeElectClnt(ts.FsLib, LEADERNAME, 0)

	for i := 0; i < N; i++ {
		go func(i int) {
			me := false
			for !me {
				err := leader.AcquireLeadership([]byte{})
				assert.Nil(ts.T, err, "AcquireLeadership")
				if current == i {
					me = true
				}
				err = leader.ReleaseLeadership()
				assert.Nil(ts.T, err, "ReleaseLeadership")
				if me {
					current += 1
					done <- i
				}
			}
		}(i)
		sum += i
	}

	for i := 0; i < N; i++ {
		next := <-done
		assert.Equal(ts.T, i, next, "Next (%v) not equal to expected (%v)", next, i)
	}

	ts.Shutdown()
}

// Test if an exit of another session doesn't remove an ephemeral
// leader of another session.
func TestEphemeralLeader(t *testing.T) {
	ts := test.MakeTstate(t)

	fsl1 := fslib.MakeFsLibAddr("fslib-1", fslib.Named())
	fsl2 := fslib.MakeFsLibAddr("fslib-2", fslib.Named())

	leader1 := electclnt.MakeElectClnt(fsl1, LEADERNAME, 0)

	err := leader1.AcquireLeadership([]byte{})
	assert.Nil(ts.T, err, "AcquireLeadership")

	// Establish a connection
	_, err = fsl2.GetFile(LEADERNAME)
	assert.Nil(ts.T, err, "GetFile")

	// Terminate connection
	fsl2.Exit()

	time.Sleep(2 * time.Second)

	err = leader1.ReleaseLeadership()
	assert.Nil(ts.T, err, "ReleaseLeadership")
	ts.Shutdown()
}
