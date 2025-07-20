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
	imgd_clnt "sigmaos/apps/imgresize/clnt"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_srv "sigmaos/ft/task/srv"
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

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/6.jpg")
	out := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img/6-thumb-xxx.jpg")
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

type TstateRPC struct {
	job     string
	mrts    *test.MultiRealmTstate
	srvProc *proc.Proc
	rpcc    *imgd_clnt.ImgResizeRPCClnt
}

func newTstateRPC(mrts *test.MultiRealmTstate) (*TstateRPC, error) {
	ts := &TstateRPC{}
	ts.mrts = mrts
	ts.job = rd.String(4)
	ts.cleanup()

	err := ts.mrts.GetRealm(test.REALM1).MkDir(sp.IMG, 0777)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	p, err := imgresize.StartImgRPCd(ts.mrts.GetRealm(test.REALM1).SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, 1, 0)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	db.DPrintf(db.TEST, "Started imgd RPC server")
	ts.srvProc = p
	rpcc, err := imgd_clnt.NewImgResizeRPCClnt(ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib, ts.job)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	ts.rpcc = rpcc
	return ts, nil
}

func (ts *TstateRPC) cleanup() {
	ts.mrts.GetRealm(test.REALM1).RmDir(sp.IMG)
	imgresize.Cleanup(ts.mrts.GetRealm(test.REALM1).FsLib, filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save"))
}

func (ts *TstateRPC) shutdown() {
	err := ts.mrts.GetRealm(test.REALM1).Evict(ts.srvProc.GetPid())
	assert.Nil(ts.mrts.T, err)
	status, err := ts.mrts.GetRealm(test.REALM1).WaitExit(ts.srvProc.GetPid())
	if assert.Nil(ts.mrts.T, err) {
		assert.True(ts.mrts.T, status.IsStatusEvicted(), "Wrong status: %v", status)
	}
}

func TestImgdRPC(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstateRPC(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	err := ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/6.jpg")
	err = ts.rpcc.Resize("resize-rpc-test", in)
	assert.Nil(ts.mrts.T, err)
	ts.shutdown()
}

type Tstate struct {
	job    string
	mrts   *test.MultiRealmTstate
	ch     chan bool
	ftsrv  *fttask_srv.FtTaskSrvMgr
	ftclnt fttask_clnt.FtTaskClnt[imgresize.Ttask, any]
}

func newTstate(mrts *test.MultiRealmTstate) (*Tstate, error) {
	ts := &Tstate{}
	ts.mrts = mrts
	ts.job = rd.String(4)
	ts.ch = make(chan bool)
	ts.cleanup()

	var err error
	ts.ftsrv, err = fttask_srv.NewFtTaskSrvMgr(ts.mrts.GetRealm(test.REALM1).SigmaClnt, fmt.Sprintf("imgresize-%s", ts.job), false)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	ts.ftclnt = fttask_clnt.NewFtTaskClnt[imgresize.Ttask, any](ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib, ts.ftsrv.Id)
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

	ts.ftsrv, err = fttask_srv.NewFtTaskSrvMgr(ts.mrts.GetRealm(test.REALM1).SigmaClnt, fmt.Sprintf("imgresize-%s", ts.job), false)
	if !assert.Nil(ts.mrts.T, err) {
		return err
	}
	ts.ftclnt = fttask_clnt.NewFtTaskClnt[imgresize.Ttask, any](ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib, ts.ftsrv.Id)

	return nil
}

func (ts *Tstate) cleanup() {
	ts.mrts.GetRealm(test.REALM1).RmDir(sp.IMG)
	imgresize.Cleanup(ts.mrts.GetRealm(test.REALM1).FsLib, filepath.Join(sp.S3, sp.ANY, "9ps3/img-save"))
}

func (ts *Tstate) shutdown() {
	ts.ch <- true
	ts.ftsrv.Stop(true)
}

func (ts *Tstate) progress() {
	for true {
		select {
		case <-ts.ch:
			return
		case <-time.After(1 * time.Second):
			if n, err := ts.ftclnt.GetNTasks(fttask_clnt.DONE); err != nil {
				assert.Nil(ts.mrts.T, err)
			} else {
				fmt.Printf("%d..", n)
			}
		}
	}
}

func (ts *Tstate) imgdJob(paths []string, em *crash.TeventMap) *spstats.TcounterSnapshot {
	imgd := imgresize.StartImgd(ts.mrts.GetRealm(test.REALM1).SigmaClnt, ts.ftclnt.ServiceId(), IMG_RESIZE_MCPU, IMG_RESIZE_MEM, false, 1, 0, em)

	tasks := make([]*fttask_clnt.Task[imgresize.Ttask], len(paths))
	for i, pn := range paths {
		tasks[i] = &fttask_clnt.Task[imgresize.Ttask]{Id: fttask_clnt.TaskId(i), Data: *imgresize.NewTask(pn)}
	}
	existing, err := ts.ftclnt.SubmitTasks(tasks)
	assert.Nil(ts.mrts.T, err)
	assert.Empty(ts.mrts.T, existing)

	db.DPrintf(db.TEST, "Submitted")

	err = ts.ftclnt.SubmittedLastTask()
	assert.Nil(ts.mrts.T, err)

	go ts.progress()

	gs := imgd.WaitGroup()
	st := spstats.NewTcounterSnapshot()
	for _, s := range gs {
		assert.True(ts.mrts.T, s.IsStatusOK(), s)
		stro, err := spstats.UnmarshalTcounterSnapshot(s.Data())
		st.MergeCounters(stro)
		assert.Nil(ts.mrts.T, err)
	}
	return st
}

func TestImgdOneOK(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	ts.imgdJob([]string{fn}, nil)
	ts.shutdown()
}

func TestImgdFatalError(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	// a non-existing file
	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/", "yyy.jpg")
	stro := ts.imgdJob([]string{fn}, nil)

	assert.True(t, stro.Counters["Nerror"] > 0)
	ts.shutdown()
}

func TestImgdOneCrash(t *testing.T) {
	e0 := crash.NewEventStart(crash.IMGRESIZE_CRASH, 100, CRASHIMG, 0.3)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	stro := ts.imgdJob([]string{fn}, crash.NewTeventMapOne(e0))
	assert.True(t, stro.Counters["Nfail"] > 0)

	ts.shutdown()
}

func TestImgdManyOK(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstate(mrts)
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
	ts.imgdJob(paths, nil)
	ts.shutdown()
}

func TestImgdRestart(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts, err1 := newTstate(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/1.jpg")

	existing, err := ts.ftclnt.SubmitTasks([]*fttask_clnt.Task[imgresize.Ttask]{{Id: 0, Data: *imgresize.NewTask(fn)}})
	assert.Nil(ts.mrts.T, err)
	assert.Empty(ts.mrts.T, existing)

	err = ts.ftclnt.SubmittedLastTask()
	assert.Nil(t, err)

	imgd := imgresize.StartImgd(ts.mrts.GetRealm(test.REALM1).SigmaClnt, ts.ftclnt.ServiceId(), IMG_RESIZE_MCPU, IMG_RESIZE_MEM, true, 1, 0, nil)

	db.DPrintf(db.TEST, "Get named contents pre-shutdown")
	sts, err := ts.mrts.GetRealm(test.REALM1).GetDir(sp.NAMED)
	assert.Nil(ts.mrts.T, err, "Err GetDir: %v", err)
	db.DPrintf(db.TEST, "%v named contents pre-shutdown: %v", test.REALM1, sp.Names(sts))

	time.Sleep(2 * time.Second)

	// Stop procgroup mgrs
	ts.ftsrv.Stop(false)
	imgd.StopGroup()

	ts.mrts.ShutdownForReboot()

	time.Sleep(sp.EtcdSessionExpired * time.Second)

	db.DPrintf(db.TEST, "Restart")

	err = ts.restartTstate()
	defer ts.mrts.Shutdown()
	if err != nil {
		return
	}

	gms, err := procgroupmgr.Recover(ts.mrts.GetRealm(test.REALM1).SigmaClnt)
	assert.Nil(ts.mrts.T, err, "Recover")
	assert.Equal(ts.mrts.T, 1, len(gms))

	go ts.progress()

	gms[0].WaitGroup()

	ts.shutdown()
}
