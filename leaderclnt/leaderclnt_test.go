package leaderclnt_test

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// "sigmaos/crash"
	// "sigmaos/fsetcd"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaderclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	leadername = "name/leader"
	dirux      = sp.UX + "/~local/outdir"
	dirnamed   = sp.NAMED + "outdir"
)

func oldleader(ts *test.Tstate, pn string, crash bool) {
	ts.MkDir(pn, 0777)
	ts.Remove(pn + "/f")
	ts.Remove(pn + "/g")

	ch := make(chan bool)
	go func() {
		fsl, err := fslib.MakeFsLibAddr("leader", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(ts.T, err, "MakeFsLib")

		l, err := leaderclnt.MakeLeaderClnt(fsl, leadername, 0777)
		assert.Nil(ts.T, err)
		err = l.LeadAndFence(nil, []string{pn})
		assert.Nil(ts.T, err, "BecomeLeaderEpoch")

		fd, err := fsl.Create(pn+"/f", 0777, sp.OWRITE)
		assert.Nil(ts.T, err, "Create")

		ch <- true

		db.DPrintf(db.TEST, "sign off as leader..\n")

		l.ReleaseLeadership()

		time.Sleep(1 * time.Second)

		db.DPrintf(db.TEST, "Try to write..\n")

		// Fsl lost primary status, and ts should have it by
		// now so this write to ux server should fail
		_, err = fsl.Write(fd, []byte(strconv.Itoa(1)))
		assert.NotNil(ts.T, err, "Write")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale))

		fsl.Close(fd)

		ch <- true
	}()

	// Wait until other thread is primary
	<-ch

	db.DPrintf(db.TEST, "Become leader..\n")

	l, err := leaderclnt.MakeLeaderClnt(ts.FsLib, leadername, 0777)
	assert.Nil(ts.T, err)
	// When other thread partitions, we become leader and start new epoch
	err = l.LeadAndFence(nil, []string{pn})
	assert.Nil(ts.T, err, "BecomeLeaderEpoch")

	// Do some op so that server becomes aware of new epoch
	_, err = ts.PutFile(pn+"/g", 0777, sp.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(ts.T, err, "PutFile")

	if crash {
		db.DPrintf(db.TEST, "kill named..\n")
		err := ts.KillOne(sp.NAMEDREL)
		assert.Nil(ts.T, err)
	}

	db.DPrintf(db.TEST, "Let old leader run..\n")

	<-ch

	fd, err := ts.Open(pn+"/f", sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	b, err := ts.Read(fd, 100)
	assert.Equal(ts.T, 0, len(b))

	sts, err := l.GetFences(pn)
	assert.Nil(ts.T, err, "GetFences")
	assert.Equal(ts.T, 1, len(sts), "Fences")

	log.Printf("fences %v\n", sp.Names(sts))

	err = l.RemoveFence([]string{pn})
	assert.Nil(ts.T, err, "RemoveFences")

	sts, err = l.GetFences(pn)
	assert.Nil(ts.T, err, "GetFences")
	assert.Equal(ts.T, 0, len(sts), "Fences")

	l.ReleaseLeadership()
}

// Test if a leader cannot write to a fenced server after leader fails
func TestOldLeaderFailUx(t *testing.T) {
	ts := test.MakeTstateAll(t)

	oldleader(ts, dirux, false)

	ts.Shutdown()
}

func TestOldLeaderFailNamed(t *testing.T) {
	ts := test.MakeTstateAll(t)

	oldleader(ts, dirnamed, false)

	ts.Shutdown()
}

func TestOldLeaderFailNamedCrash(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.Boot(sp.NAMEDREL)
	assert.Nil(t, err)

	oldleader(ts, dirnamed, true)

	ts.Shutdown()
}
