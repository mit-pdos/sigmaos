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
	"sigmaos/util/crash"
)

//
// For testing
//

const (
	leadername = "name/leader"
)

func OldleaderTest(ts *test.Tstate, pn string, crashfn string, realm sp.Trealm) *LeaderClnt {
	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), realm)
	fsl, err := sigmaclnt.NewFsLib(pe, ts.GetDialProxyClnt())
	assert.Nil(ts.T, err, "NewFsLib")
	fsl.MkDir(pn, 0777)
	fsl.Remove(pn + "/fff")
	fsl.Remove(pn + "/ggg")
	fsl.Remove(pn + "/sss")
	fsl.Remove(pn + "/ttt")
	fsl.Remove(pn + "/uuu")

	ch := make(chan bool)
	go func() {
		// Make a new fsl for this test, because we want to use ts.FsLib
		// to shutdown the system.
		//		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), realm)
		fsl2, err := sigmaclnt.NewFsLib(pe, ts.GetDialProxyClnt())
		assert.Nil(ts.T, err, "NewFsLib")

		l, err := NewLeaderClnt(fsl2, leadername, 0777)
		assert.Nil(ts.T, err)
		err = l.LeadAndFence(nil, []string{pn})
		assert.Nil(ts.T, err, "BecomeLeaderEpoch")

		_, err = fsl2.PutFile(pn+"/sss", 0777, sp.OWRITE, nil)
		assert.Nil(ts.T, err, "PutFile")

		fd, err := fsl2.Create(pn+"/fff", 0777, sp.OWRITE)
		assert.Nil(ts.T, err, "Create")

		fence := l.Fence()

		ch <- true

		db.DPrintf(db.TEST, "sign off as leader %v...", fence)

		l.ReleaseLeadership()

		<-ch

		db.DPrintf(db.TEST, "Old leader try to write...")

		// A leader shouldn't write after resigning, but this is for
		// testing that operations of an old leader fails.  This
		// thread lost leader status, and the other thread should have
		// it by now so this write to pn should fail, because it is
		// fenced with the fsl's fence, which is the old leader's one.

		_, err = fsl2.PutFile(pn+"/fff", 0777, sp.OWRITE, []byte("should fail"))
		assert.NotNil(ts.T, err, "Put")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale), "Err code: %v", err)
		fsl2.CloseFd(fd)

		fd, err = fsl2.Create(pn+"/uuu", 0777, sp.OWRITE)
		assert.NotNil(ts.T, err, "Create")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale), "Err code: %v", err)
		fsl2.CloseFd(fd)

		fd, err = fsl2.CreateLeased(pn+"/uuu", 0777, sp.OWRITE, sp.NoLeaseId, fence)
		assert.NotNil(ts.T, err, "CreateLeased")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale), "Err code: %v", err)
		fsl2.CloseFd(fd)

		_, err = fsl2.PutFile(pn+"/uuu", 0777, sp.OWRITE, nil)
		assert.NotNil(ts.T, err, "PutFile")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale), "Err code: %v", err)

		err = fsl2.Rename(pn+"/sss", pn+"/ttt")
		assert.NotNil(ts.T, err, "Rename")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale), "Err code: %v", err)
		fsl2.CloseFd(fd)

		err = fsl2.Remove(pn + "/sss")
		assert.NotNil(ts.T, err, "Remove")
		assert.True(ts.T, serr.IsErrCode(err, serr.TErrStale), "Err code: %v", err)
		fsl2.CloseFd(fd)

		ch <- true

		fsl2.Close()
	}()

	// Wait until other thread is leader and resigns
	<-ch

	db.DPrintf(db.TEST, "Become leader...")

	l, err := NewLeaderClnt(fsl, leadername, 0777)
	assert.Nil(ts.T, err)
	// When other thread resigns, we become leader and start new epoch
	err = l.LeadAndFence(nil, []string{pn})
	assert.Nil(ts.T, err, "BecomeLeaderEpoch")

	db.DPrintf(db.TEST, "fence new leader %v", l.Fence())

	// Do some op so that server becomes aware of new epoch
	_, err = fsl.PutFile(pn+"/ggg", 0777, sp.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(ts.T, err, "PutFile")

	if crashfn != "" {
		db.DPrintf(db.TEST, "Crash named...")
		err := crash.SignalFailer(fsl, crashfn)
		assert.Nil(ts.T, err, "Err failer: %v", err)
	}

	// let old leader run
	ch <- true

	db.DPrintf(db.TEST, "Wait until old leader is done...")

	<-ch

	fd, err := fsl.Open(pn+"/fff", sp.OREAD)
	assert.Nil(ts.T, err, "Open err %v", err)
	b := make([]byte, 100)
	cnt, err := fsl.Read(fd, b)
	assert.Equal(ts.T, sp.Tsize(0), cnt, "buf %v", string(b))

	_, err = fsl.Stat(pn + "/sss")
	assert.Nil(ts.T, err, "Stat err %v", err)

	return l
}
