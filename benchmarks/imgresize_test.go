package benchmarks_test

import (
	"path"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/groupmgr"
	"sigmaos/imgresized"
	"sigmaos/perf"
	"sigmaos/proc"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type ImgResizeJobInstance struct {
	sigmaos  bool
	job      string
	imgdmcpu proc.Tmcpu
	mcpu     proc.Tmcpu
	mem      proc.Tmem
	ntasks   int
	ninputs  int
	nrounds  int
	input    string
	ready    chan bool
	imgd     *groupmgr.GroupMgr
	p        *perf.Perf
	*test.RealmTstate
}

func NewImgResizeJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, ntasks int, ninputs int, mcpu proc.Tmcpu, mem proc.Tmem, nrounds int, imgdmcpu proc.Tmcpu) *ImgResizeJobInstance {
	ji := &ImgResizeJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = "imgresize-" + ts.GetRealm().String() + "-" + rd.String(4)
	ji.ntasks = ntasks
	ji.input = input
	ji.ready = make(chan bool)
	ji.RealmTstate = ts
	ji.p = p
	ji.ninputs = ninputs
	ji.mcpu = mcpu
	ji.imgdmcpu = imgdmcpu
	ji.mem = mem
	ji.nrounds = nrounds

	err := imgresized.MkDirs(ji.FsLib, ji.job)
	assert.Nil(ts.Ts.T, err, "Error MkDirs: %v", err)

	fn := ji.input
	fns := make([]string, 0, ji.ninputs)
	for i := 0; i < ji.ninputs; i++ {
		fns = append(fns, fn)
	}

	db.DPrintf(db.ALWAYS, "Submit ImgResizeJob tasks")
	for i := 0; i < ji.ntasks; i++ {
		err := imgresized.SubmitTaskMulti(ji.SigmaClnt.FsLib, ji.job, fns)
		assert.Nil(ji.Ts.T, err, "Error SubmitTask: %v", err)
	}
	db.DPrintf(db.ALWAYS, "Done submitting ImgResize tasks")

	return ji
}

func (ji *ImgResizeJobInstance) StartImgResizeJob() {
	db.DPrintf(db.ALWAYS, "StartImgResizeJob input %v ntasks %v mcpu %v job %v", ji.input, ji.ntasks, ji.mcpu, ji.job)
	ji.imgd = imgresized.StartImgd(ji.SigmaClnt, ji.job, ji.mcpu, ji.mem, false, ji.nrounds, ji.imgdmcpu)
	db.DPrintf(db.ALWAYS, "Done starting ImgResizeJob")
}

func (ji *ImgResizeJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeJOb to finish")
	for {
		n, err := imgresized.NTaskDone(ji.SigmaClnt.FsLib, ji.job)
		assert.Nil(ji.Ts.T, err, "Error NTaskDone: %v", err)
		db.DPrintf(db.TEST, "ImgResizeJob NTaskDone: %v", n)
		if n == ji.ntasks {
			break
		}
		time.Sleep(1 * time.Second)
	}
	db.DPrintf(db.TEST, "Done waiting for ImgResizeJob to finish")
	ji.imgd.StopGroup()
	db.DPrintf(db.TEST, "Imgd shutdown")
}

func (ji *ImgResizeJobInstance) Cleanup() {
	dir := path.Join(sp.UX, "~local", path.Dir(ji.input))
	db.DPrintf(db.TEST, "Cleaning up dir %v", dir)
	imgresized.Cleanup(ji.FsLib, dir)
}
