package imgresize_test

import (
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nfnt/resize"
	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	rd "sigmaos/util/rand"
	"sigmaos/util/spstats"
)

const (
	IMG_RESIZE_MCPU proc.Tmcpu = 100
	IMG_RESIZE_MEM  proc.Tmem  = 0

	CRASHIMG = 1000
)

func TestCompile(t *testing.T) {
}

func TestResizeImg(t *testing.T) {
	fn := "/tmp/thumb.jpeg"

	os.Remove(fn)

	in, err := os.Open("1.jpg")
	assert.Nil(t, err)
	img, err := jpeg.Decode(in)
	assert.Nil(t, err)

	start := time.Now()

	img1 := resize.Resize(160, 0, img, resize.Lanczos3)

	db.DPrintf(db.TEST, "resize %v\n", time.Since(start))

	out, err := os.Create(fn)
	assert.Nil(t, err)
	jpeg.Encode(out, img1, nil)
}

func TestResizeProc(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	out := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img/8-thumb-xxx.jpg")
	mrts.GetRealm(test.REALM1).Remove(out)
	p := proc.NewProc("imgresize", []string{in, out, "1"})
	err := mrts.GetRealm(test.REALM1).Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error %v", err)
	assert.True(t, status.IsStatusOK(), "WaitExit status error: %v", status)
}

type Tstate struct {
	job  string
	mrts *test.MultiRealmTstate
	ch   chan bool
	imgd *imgresize.ImgdMgr[imgresize.Ttask]
	clnt *imgresize.ImgdClnt[imgresize.Ttask]
}

func newTstate(mrts *test.MultiRealmTstate, persist bool, em *crash.TeventMap) (*Tstate, error) {
	ts := &Tstate{}
	ts.mrts = mrts
	ts.job = rd.String(4)
	ts.ch = make(chan bool)
	ts.cleanup()

	imgd, err := imgresize.NewImgdMgr[imgresize.Ttask](ts.mrts.GetRealm(test.REALM1).SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, persist, 1, 0, em)
	if err != nil {
		return nil, err
	}
	ts.imgd = imgd

	clnt, err := imgd.NewImgdClnt(ts.mrts.GetRealm(test.REALM1).SigmaClnt)
	if err != nil {
		return nil, err
	}
	ts.clnt = clnt

	return ts, nil
}

func (ts *Tstate) restartTstate() error {
	mrts, err1 := test.NewMultiRealmTstate(ts.mrts.T, []sp.Trealm{test.REALM1})
	if !assert.Nil(ts.mrts.T, err1, "Error New Tstate: %v", err1) {
		return err1
	}

	ts.mrts = mrts
	db.DPrintf(db.TEST, "Get named contents post-shutdown")

	sts, err := ts.mrts.GetRealm(test.REALM1).GetDir(sp.NAMED)
	if !assert.Nil(ts.mrts.T, err, "Err GetDir: %v", err) {
		return err
	}

	db.DPrintf(db.TEST, "%v named contents post-shutdown: %v", test.REALM1, sp.Names(sts))
	sc := ts.mrts.GetRealm(test.REALM1).SigmaClnt

	if err := ts.imgd.Restart(sc); err != nil {
		return err
	}

	clnt, err := ts.imgd.NewImgdClnt(sc)
	if err != nil {
		return err
	}
	ts.clnt = clnt

	return nil
}

func (ts *Tstate) cleanup() {
	ts.mrts.GetRealm(test.REALM1).RmDir(sp.IMG)
	imgresize.Cleanup(ts.mrts.GetRealm(test.REALM1).FsLib, filepath.Join(sp.S3, sp.ANY, "9ps3/img-save"))
}

func (ts *Tstate) stopProgress() {
	ts.ch <- true
}

func (ts *Tstate) progress() {
	for true {
		select {
		case <-ts.ch:
			return
		case <-time.After(1 * time.Second):
			if n, err := ts.clnt.GetNTasks(fttask_clnt.DONE); err != nil {
				assert.Nil(ts.mrts.T, err)
			} else {
				fmt.Printf("%d..", n)
			}
		}
	}
}

func TestImgdResizeRPC(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts, false, nil)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	err := ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	go ts.progress()

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	err = ts.clnt.Resize("resize-rpc-test", in)
	assert.Nil(ts.mrts.T, err)

	n, err := ts.clnt.Status()
	assert.Nil(ts.mrts.T, err)
	assert.Equal(ts.mrts.T, int64(1), n)

	sts, err := ts.imgd.StopImgd(true)
	assert.Nil(ts.mrts.T, err)
	for _, st := range sts {
		assert.True(ts.mrts.T, st.IsStatusEvicted())
	}

	ts.stopProgress()
}

func (ts *Tstate) doJob(paths []string) *spstats.TcounterSnapshot {
	tasks := make([]*fttask_clnt.Task[imgresize.Ttask], len(paths))
	for i, pn := range paths {
		tasks[i] = &fttask_clnt.Task[imgresize.Ttask]{Id: fttask_clnt.TaskId(i), Data: *imgresize.NewTask(pn)}
	}
	err := ts.clnt.SubmitTasks(tasks)
	assert.Nil(ts.mrts.T, err)

	db.DPrintf(db.TEST, "Submitted")

	err = ts.clnt.SubmittedLastTask()
	assert.Nil(ts.mrts.T, err)

	go ts.progress()

	gs := ts.imgd.WaitImgd()
	st := spstats.NewTcounterSnapshot()
	for _, s := range gs {
		assert.True(ts.mrts.T, s.IsStatusOK(), s)
		stro, err := spstats.UnmarshalTcounterSnapshot(s.Data())
		st.MergeCounters(stro)
		assert.Nil(ts.mrts.T, err)
	}

	ts.stopProgress()
	return st
}

func TestImgdOneOK(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts, false, nil)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	ts.doJob([]string{fn})
}

func TestImgdFatalError(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts, false, nil)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	// a non-existing file
	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/", "yyy.jpg")
	stro := ts.doJob([]string{fn})

	assert.True(t, stro.Counters["Nerror"] > 0)
}

func TestImgdManyOK(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts, false, nil)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	err := ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	sts, err1 := ts.mrts.GetRealm(test.REALM1).GetDir(filepath.Join(sp.S3, sp.ANY, "9ps3/img-save"))
	assert.Nil(t, err)

	paths := make([]string, 0, len(sts))
	for _, st := range sts {
		fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/", st.Name)
		paths = append(paths, fn)
	}
	ts.doJob(paths)
}

func TestImgdProcCrash(t *testing.T) {
	e0 := crash.NewEventStart(crash.IMGRESIZE_CRASH, 100, CRASHIMG, 0.3)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts, false, crash.NewTeventMapOne(e0))
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	stro := ts.doJob([]string{fn})
	assert.True(t, stro.Counters["Nfail"] > 0)
}

func TestImgdRestart(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts, err1 := newTstate(mrts, true, nil)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")

	err := ts.clnt.SubmitTasks([]*fttask_clnt.Task[imgresize.Ttask]{{Id: 0, Data: *imgresize.NewTask(fn)}})
	assert.Nil(ts.mrts.T, err)

	err = ts.clnt.SubmittedLastTask()
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Get named contents pre-shutdown")
	sts, err := ts.mrts.GetRealm(test.REALM1).GetDir(sp.NAMED)
	assert.Nil(ts.mrts.T, err, "Err GetDir: %v", err)
	db.DPrintf(db.TEST, "%v named contents pre-shutdown: %v", test.REALM1, sp.Names(sts))

	time.Sleep(2 * time.Second)

	ts.imgd.StopImgd(false)

	ts.mrts.ShutdownForReboot()

	time.Sleep(sp.EtcdSessionExpired * time.Second)

	db.DPrintf(db.TEST, "Restart")

	err = ts.restartTstate()
	defer ts.mrts.Shutdown()
	if err != nil {
		return
	}

	go ts.progress()

	ts.imgd.WaitImgd()

	ts.stopProgress()
}
