package benchmarks_test

import (
	"path"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/groupmgr"
	"sigmaos/imgresized"
	"sigmaos/perf"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type ImgResizeJobInstance struct {
	sigmaos bool
	job     string
	ntasks  int
	input   string
	ready   chan bool
	imgd    *groupmgr.GroupMgr
	p       *perf.Perf
	*test.RealmTstate
}

func MakeImgResizeJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, ntasks int) *ImgResizeJobInstance {
	ji := &ImgResizeJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = "imgresize-" + rd.String(4)
	ji.ntasks = ntasks
	ji.input = input
	ji.ready = make(chan bool)
	ji.RealmTstate = ts
	ji.p = p

	err := imgresized.MkDirs(ji.FsLib, ji.job)
	assert.Nil(ts.T, err, "Error MkDirs: %v", err)

	return ji
}

func (ji *ImgResizeJobInstance) StartImgResizeJob() {
	db.DPrintf(db.ALWAYS, "StartImgResizeJob input %v ntasks %v", ji.input, ji.ntasks)
	ji.imgd = imgresized.StartImgd(ji.SigmaClnt, ji.job)
	fn := path.Join(sp.S3, "~local", ji.input)
	for i := 0; i < ji.ntasks; i++ {
		err := imgresized.SubmitTask(ji.SigmaClnt.FsLib, ji.job, fn)
		assert.Nil(ji.T, err, "Error SubmitTask: %v", err)
	}
	db.DPrintf(db.ALWAYS, "Done starting ImgResizeJob")
}

func (ji *ImgResizeJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeJOb to finish")
	for {
		n, err := imgresized.NTaskDone(ji.SigmaClnt.FsLib, ji.job)
		assert.Nil(ji.T, err, "Error NTaskDone: %v", err)
		db.DPrintf(db.TEST, "ImgResizeJob NTaskDone: %v", n)
		if n == ji.ntasks {
			break
		}
		time.Sleep(1 * time.Second)
	}
	db.DPrintf(db.TEST, "Done waiting for ImgResizeJob to finish")
	ji.imgd.Wait()
	db.DPrintf(db.TEST, "Imgd shutdown")
}

func (ji *ImgResizeJobInstance) Cleanup() {
	dir := path.Join(sp.S3, "~local", path.Dir(ji.input))
	db.DPrintf(db.TEST, "Cleaning up dir %v", dir)
	imgresized.Cleanup(ji.FsLib, dir)
}
