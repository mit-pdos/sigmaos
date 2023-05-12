package imgresized_test

import (
	"fmt"
	"image/jpeg"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/nfnt/resize"
	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	"sigmaos/groupmgr"
	"sigmaos/imgresized"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestResizeImg(t *testing.T) {
	fn := "/tmp/thumb.jpeg"

	os.Remove(fn)

	in, err := os.Open("/home/kaashoek/Downloads/desk.jpg")
	assert.Nil(t, err)
	img, err := jpeg.Decode(in)
	assert.Nil(t, err)

	start := time.Now()

	img1 := resize.Resize(160, 0, img, resize.Lanczos3)

	fmt.Printf("resize %v\v", time.Since(start))

	out, err := os.Create(fn)
	assert.Nil(t, err)
	jpeg.Encode(out, img1, nil)
}

func TestResizeProc(t *testing.T) {
	ts := test.MakeTstateAll(t)
	in := path.Join(sp.S3, "~local/9ps3/desk.jpg")
	out := path.Join(sp.S3, "~local/9ps3/thumb.jpg")
	ts.Remove(out)
	p := proc.MakeProc("imgresize", []string{in, out})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(p.GetPid())
	assert.True(t, status.IsStatusOK(), "WaitExit status error")
	ts.Shutdown()
}

type Tstate struct {
	job string
	*test.Tstate
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.job = rd.String(4)
	return ts
}

func (ts *Tstate) WaitDone() {
	for true {
		time.Sleep(1 * time.Second)
		if n, err := imgresized.NTask(ts.SigmaClnt.FsLib, ts.job); err != nil {
			break
		} else if n == 0 {
			break
		} else {
			fmt.Printf("%d..", n)
		}
	}
	fmt.Printf("\n")
}

func startImgd(sc *sigmaclnt.SigmaClnt, job string) *groupmgr.GroupMgr {
	return groupmgr.Start(sc, 1, "imgresized", []string{strconv.Itoa(0)}, job, 0, 1, 0, 0, 0)
}

func TestImgdOne(t *testing.T) {
	ts := makeTstate(t)

	err := imgresized.MkDirs(ts.SigmaClnt.FsLib, ts.job)
	assert.Nil(t, err)

	fn := path.Join(sp.S3, "~local/9ps3/img/1.jpg")
	ts.Remove(imgresized.ThumbName(fn))

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, fn)
	assert.Nil(t, err)

	imgd := startImgd(ts.SigmaClnt, ts.job)

	ts.WaitDone()

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, imgresized.STOP)
	assert.Nil(t, err)

	imgd.Wait()

	ts.Shutdown()
}

func TestImgdMany(t *testing.T) {
	ts := makeTstate(t)

	err := imgresized.MkDirs(ts.SigmaClnt.FsLib, ts.job)
	assert.Nil(t, err)

	imgd := startImgd(ts.SigmaClnt, ts.job)

	sts, err := ts.GetDir(path.Join(sp.S3, "~local/9ps3/img"))
	assert.Nil(t, err)

	for _, st := range sts {
		fn := path.Join(sp.S3, "~local/9ps3/img/", st.Name)
		ts.Remove(imgresized.ThumbName(fn))
	}

	sts, err = ts.GetDir(path.Join(sp.S3, "~local/9ps3/img"))
	assert.Nil(t, err)

	for _, st := range sts {
		fmt.Printf("submit %v\n", st.Name)
		fn := path.Join(sp.S3, "~local/9ps3/img/", st.Name)
		err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, fn)
		assert.Nil(t, err)
	}

	ts.WaitDone()

	err = imgresized.SubmitTask(ts.SigmaClnt.FsLib, ts.job, imgresized.STOP)
	assert.Nil(t, err)

	imgd.Wait()

	ts.Shutdown()
}
