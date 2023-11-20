package imgresized_test

import (
	"fmt"
	"image/jpeg"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nfnt/resize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/groupmgr"
	"sigmaos/imgresized"
	"sigmaos/proc"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	IMG_RESIZE_MCPU proc.Tmcpu = 100
	IMG_RESIZE_MEM  proc.Tmem  = 0
)

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
	ts := test.NewTstateAll(t)
	in := path.Join(sp.S3, "~local/9ps3/img-save/6.jpg")
	//	in := path.Join(sp.S3, "~local/9ps3/img-save/6.jpg")
	out := path.Join(sp.S3, "~local/9ps3/img/6-thumb-xxx.jpg")
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
}

func newTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.NewTstateAll(t)
	ts.job = rd.String(4)
	ts.ch = make(chan bool)
	ts.cleanup()
	return ts
}

func (ts *Tstate) cleanup() {
	imgresized.Cleanup(ts.FsLib, path.Join(sp.S3, "~local/9ps3/img-save"))
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
			if n, err := imgresized.NTaskDone(ts.SigmaClnt.FsLib, ts.job); err != nil {
				assert.Nil(ts.T, err)
			} else {
				fmt.Printf("%d..", n)
			}
		}
	}
}

func TestImgdFatal(t *testing.T) {
	ts := newTstate(t)

	err := imgresized.MkDirs(ts.SigmaClnt.FsLib, ts.job)
	assert.Nil(ts.T, err)

	imgd := imgresized.StartImgd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, false, 1)
	fn := path.Join(sp.S3, "~local/9ps3/img-save/", "yyy.jpg")

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, fn)
	assert.Nil(ts.T, err)

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, imgresized.STOP)
	assert.Nil(ts.T, err)

	gs := imgd.WaitGroup()
	for _, s := range gs {
		assert.True(ts.T, s.IsStatusFatal(), s)
	}
	db.DPrintf(db.TEST, "shutdown\n")
	ts.Shutdown()
}

func (ts *Tstate) imgdJob(paths []string) {
	err := imgresized.MkDirs(ts.SigmaClnt.FsLib, ts.job)
	assert.Nil(ts.T, err)

	imgd := imgresized.StartImgd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, false, 1)

	for _, pn := range paths {
		db.DPrintf(db.TEST, "submit %v\n", pn)
		err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, pn)
		assert.Nil(ts.T, err)
	}

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, imgresized.STOP)
	assert.Nil(ts.T, err)

	go ts.progress()

	gs := imgd.WaitGroup()
	for _, s := range gs {
		assert.True(ts.T, s.IsStatusOK(), s)
	}
}

func TestImgdOne(t *testing.T) {
	ts := newTstate(t)
	fn := path.Join(sp.S3, "~local/9ps3/img-save/1.jpg")
	ts.imgdJob([]string{fn})
	ts.shutdown()
}

func TestImgdMany(t *testing.T) {
	ts := newTstate(t)

	sts, err := ts.GetDir(path.Join(sp.S3, "~local/9ps3/img-save"))
	assert.Nil(t, err)

	paths := make([]string, 0, len(sts))
	for _, st := range sts {
		fn := path.Join(sp.S3, "~local/9ps3/img-save/", st.Name)
		paths = append(paths, fn)
	}

	ts.imgdJob(paths)
	ts.shutdown()
}

func TestImgdRestart(t *testing.T) {
	ts := newTstate(t)

	err := imgresized.MkDirs(ts.SigmaClnt.FsLib, ts.job)
	assert.Nil(t, err)

	fn := path.Join(sp.S3, "~local/9ps3/img-save/1.jpg")

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, fn)
	assert.Nil(t, err)

	imgd := imgresized.StartImgd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, true, 1)

	time.Sleep(2 * time.Second)

	imgd.StopGroup()

	ts.Shutdown()

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	db.DPrintf(db.TEST, "Restart")

	ts.Tstate = test.NewTstateAll(t)

	gms, err := groupmgr.Recover(ts.SigmaClnt)
	assert.Nil(ts.T, err, "Recover")
	assert.Equal(ts.T, 1, len(gms))

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, imgresized.STOP)
	assert.Nil(t, err)

	go ts.progress()

	gms[0].WaitGroup()

	ts.shutdown()
}
