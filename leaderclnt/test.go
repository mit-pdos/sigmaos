package leaderclnt

import (
	"strconv"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

//
// For testing
//

const (
	leadername = "name/leader"
)

func OldleaderTest(ts *test.Tstate, pn string, crash bool) *LeaderClnt {
	ts.MkDir(pn, 0777)
	ts.Remove(pn + "/f")
	ts.Remove(pn + "/g")

	ch := make(chan bool)
	go func() {
		// Make a new fsl for this test, because we want to use ts.FsLib
		// to shutdown the system.
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl2, err := sigmaclnt.NewFsLib(pe, ts.GetNetProxyClnt())
		assert.Nil(ts.T, err, "NewFsLib")

		l, err := NewLeaderClnt(fsl2, leadername, 0777)
		assert.Nil(ts.T, err)
		err = l.LeadAndFence(nil, []string{pn})
		assert.Nil(ts.T, err, "BecomeLeaderEpoch")

		fd, err := fsl2.Create(pn+"/f", 0777, sp.OWRITE)
		assert.Nil(ts.T, err, "Create")

		ch <- true

		db.DPrintf(db.TEST, "sign off as leader..\n")

		l.ReleaseLeadership()

		<-ch

		db.DPrintf(db.TEST, "Old leader try to write..\n")

		// A thread shouldn't write after resigning, but this thread
		// lost leader status, and the other thread should have it by
		// now so this write to pn should fail, because it is fenced
		// with the fsl's fence, which is the old leader's one.

		_, err = fsl2.PutFile(pn+"/f", 0777, sp.OWRITE, []byte("should fail"))
		assert.NotNil(ts.T, err, "Put")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale))
		fsl2.CloseFd(fd)

		ch <- true

		fsl2.Close()
	}()

	// Wait until other thread is leader and resigns
	<-ch

	db.DPrintf(db.TEST, "Become leader..\n")

	l, err := NewLeaderClnt(ts.FsLib, leadername, 0777)
	assert.Nil(ts.T, err)
	// When other thread resigns, we become leader and start new epoch
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

	// let old leader run
	ch <- true

	db.DPrintf(db.TEST, "Wait until old leader is done..\n")

	<-ch

	fd, err := ts.Open(pn+"/f", sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	b := make([]byte, 100)
	cnt, err := ts.Read(fd, b)
	assert.Equal(ts.T, sp.Tsize(0), cnt, "buf %v", string(b))

	return l
}
