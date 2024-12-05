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
	"sigmaos/ft/groupmgr"
	fttask "sigmaos/ft/task"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	rd "sigmaos/util/rand"
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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/6.jpg")
	out := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img/6-thumb-xxx.jpg")
	ts.Remove(out)
	p := proc.NewProc("imgresize", []string{in, out, "1"})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error %v", err)
	assert.True(t, status.IsStatusOK(), "WaitExit status error: %v", status)
	ts.Shutdown()
}

type Tstate struct {
	job string
	*test.Tstate
	ch chan bool
	ft *fttask.FtTasks
}

func newTstate(t *test.Tstate) (*Tstate, error) {
	ts := &Tstate{}
	ts.Tstate = t
	ts.job = rd.String(4)
	ts.ch = make(chan bool)
	ts.cleanup()

	ft, err := fttask.MkFtTasks(ts.SigmaClnt.FsLib, sp.IMG, ts.job)
	if !assert.Nil(ts.T, err) {
		return nil, err
	}
	ts.ft = ft
	return ts, nil
}

func (ts *Tstate) restartTstate() {
	ts1, err := test.NewTstateAll(ts.T)
	if !assert.Nil(ts.T, err, "Error New Tstate: %v", err) {
		return
	}
	ts.Tstate = ts1
	ft, err := fttask.NewFtTasks(ts.SigmaClnt.FsLib, sp.IMG, ts.job)
	assert.Nil(ts.T, err)
	ts.ft = ft
}

func (ts *Tstate) cleanup() {
	ts.RmDir(sp.IMG)
	imgresize.Cleanup(ts.FsLib, filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save"))
}

func (ts *Tstate) shutdown() {
	ts.ch <- true
	ts.Shutdown()
}

func (ts *Tstate) progress() {
	for true {
		select {
		case <-ts.ch:
			return
		case <-time.After(1 * time.Second):
			if n, err := ts.ft.NTaskDone(); err != nil {
				assert.Nil(ts.T, err)
			} else {
				fmt.Printf("%d..", n)
			}
		}
	}
}

func TestImgdFatalError(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer t1.Shutdown()
	ts, err1 := newTstate(t1)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	imgd := imgresize.StartImgd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, false, 1, 0, nil)

	// a non-existing file
	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/", "yyy.jpg")

	err := ts.ft.SubmitTask(0, imgresize.NewTask(fn))
	assert.Nil(ts.T, err)

	err = ts.ft.SubmitStop()
	assert.Nil(ts.T, err)

	gs := imgd.WaitGroup()
	for _, s := range gs {
		assert.True(ts.T, s.IsStatusFatal(), s)
	}
}

func (ts *Tstate) imgdJob(paths []string, em *crash.TeventMap) {
	imgd := imgresize.StartImgd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, false, 1, 0, em)

	for i, pn := range paths {
		db.DPrintf(db.TEST, "submit %v\n", pn)
		err := ts.ft.SubmitTask(i, imgresize.NewTask(pn))
		assert.Nil(ts.T, err)
	}

	err := ts.ft.SubmitStop()
	assert.Nil(ts.T, err)

	go ts.progress()

	gs := imgd.WaitGroup()
	for _, s := range gs {
		assert.True(ts.T, s.IsStatusOK(), s)
	}
}

func TestImgdOneOK(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, err1 := newTstate(t1)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		t1.Shutdown()
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	ts.imgdJob([]string{fn}, nil)
	ts.shutdown()
}

func TestImgdOneCrash(t *testing.T) {
	e0 := crash.NewEventStart(crash.IMGRESIZE_CRASH, 100, CRASHIMG, 0.3)

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, err1 := newTstate(t1)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		t1.Shutdown()
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/8.jpg")
	ts.imgdJob([]string{fn}, crash.NewTeventMapOne(e0))
	ts.shutdown()
}

func TestImgdManyOK(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, err1 := newTstate(t1)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		t1.Shutdown()
		return
	}

	err := ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	sts, err1 := ts.GetDir(filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save"))
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
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, err1 := newTstate(t1)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		t1.Shutdown()
		return
	}

	fn := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/1.jpg")

	err := ts.ft.SubmitTask(0, imgresize.NewTask(fn))
	assert.Nil(t, err)

	imgd := imgresize.StartImgd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, true, 1, 0, nil)

	time.Sleep(2 * time.Second)

	imgd.StopGroup()

	ts.Shutdown()

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	db.DPrintf(db.TEST, "Restart")

	ts.restartTstate()

	gms, err := groupmgr.Recover(ts.SigmaClnt)
	assert.Nil(ts.T, err, "Recover")
	assert.Equal(ts.T, 1, len(gms))

	err = ts.ft.SubmitStop()
	assert.Nil(t, err)

	go ts.progress()

	gms[0].WaitGroup()

	ts.shutdown()
}
