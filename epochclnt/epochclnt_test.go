package epochclnt_test

import (
	"log"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/crash"
	"ulambda/delay"
	"ulambda/epochclnt"
	"ulambda/fenceclnt1"
	"ulambda/fslib"
	"ulambda/leaderclnt"
	np "ulambda/ninep"
	"ulambda/test"
)

const (
	leadername = "name/leader"
	epochname  = leadername + "-epoch"
	dirux      = np.UX + "/~ip/outdir"
)

func becomeLeader(t *testing.T, fsl *fslib.FsLib) {
	leader := leaderclnt.MakeLeaderClnt(fsl, leadername, 0)
	err := leader.AcquireLeadership()
	assert.Nil(t, err, "AcquireLeadership")
	ec := epochclnt.MakeEpochClnt(fsl, epochname, 0777)
	fc := fenceclnt1.MakeFenceClnt(fsl, ec, 0777, []string{dirux})
	epoch, err := ec.AdvanceEpoch()
	assert.Nil(t, err, "AdvanceEpoch")
	log.Printf("leader at %v\n", epoch)
	err = fc.FenceAtEpoch(epoch)
	assert.Nil(t, err, "FenceAtEpoch")
}

// Test if a leader cannot write to a fenced server after leader fails
func TestOldLeaderFail(t *testing.T) {
	ts := test.MakeTstateAll(t)

	ts.MkDir(dirux, 0777)
	ts.Remove(dirux + "/f")
	ts.Remove(dirux + "/g")

	_, err := ts.PutFile(epochname, 0777, np.OWRITE, []byte{})
	assert.Nil(t, err, "PutFile")

	fsl := fslib.MakeFsLibAddr("leader", fslib.Named())

	ch := make(chan bool)
	go func() {
		becomeLeader(t, fsl)

		fd, err := fsl.Create(dirux+"/f", 0777, np.OWRITE)
		assert.Nil(t, err, "Create")

		ch <- true

		log.Printf("partition from named..\n")

		crash.Partition(fsl)
		delay.Delay(10)

		// fsl lost primary status, and ts should have it by
		// now so this write to ux server should fail
		_, err = fsl.Write(fd, []byte(strconv.Itoa(1)))
		assert.NotNil(t, err, "Write")

		fsl.Close(fd)

		ch <- true
	}()

	// Wait until other thread is primary
	<-ch

	// When other thread partitions, we become leader and start new epoch
	becomeLeader(t, ts.FsLib)

	// Do some op so that server becomes aware of new epoch
	_, err = ts.PutFile(dirux+"/g", 0777, np.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(t, err, "PutFile")

	<-ch

	fd, err := ts.Open(dirux+"/f", np.OREAD)
	assert.Nil(t, err, "Open")
	b, err := ts.Read(fd, 100)
	assert.Equal(ts.T, 0, len(b))

	ts.Shutdown()
}
