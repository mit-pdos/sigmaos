package benchmarks_test

import (
	"path/filepath"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	"sigmaos/fttasks"
	"sigmaos/groupmgr"
	"sigmaos/util/perf"
	"sigmaos/proc"
	rd "sigmaos/util/rand"
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
	ft       *fttasks.FtTasks
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

	ts.RmDir(imgresize.IMG)

	ft, err := fttasks.MkFtTasks(ji.SigmaClnt.FsLib, imgresize.IMG, ji.job)
	assert.Nil(ts.Ts.T, err, "Error MkDirs: %v", err)
	ji.ft = ft

	fn := ji.input
	fns := make([]string, 0, ji.ninputs)
	for i := 0; i < ji.ninputs; i++ {
		fns = append(fns, fn)
	}

	db.DPrintf(db.ALWAYS, "Submit ImgResizeJob tasks")
	for i := 0; i < ji.ntasks; i++ {
		ts := make([]interface{}, 0, len(fns))
		for _, fn := range fns {
			ts = append(ts, imgresize.NewTask(fn))
		}
		err := ft.SubmitTaskMulti(i, ts)
		assert.Nil(ji.Ts.T, err, "Error SubmitTask: %v", err)
	}
	// Sanity check
	n, err := ft.NTasksTODO()
	assert.Nil(ji.Ts.T, err, "Error NTasksTODO: %v", err)
	assert.Equal(ji.Ts.T, n, ji.ntasks, "Num tasks TODO doesn't match ntasks")
	db.DPrintf(db.ALWAYS, "Done submitting ImgResize tasks")
	return ji
}

func (ji *ImgResizeJobInstance) StartImgResizeJob() {
	db.DPrintf(db.ALWAYS, "StartImgResizeJob input %v ntasks %v mcpu %v job %v", ji.input, ji.ntasks, ji.mcpu, ji.job)
	ji.imgd = imgresize.StartImgd(ji.SigmaClnt, ji.job, ji.mcpu, ji.mem, false, ji.nrounds, ji.imgdmcpu)
	db.DPrintf(db.ALWAYS, "Done starting ImgResizeJob")
}

func (ji *ImgResizeJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeJOb to finish")
	for {
		n, err := ji.ft.NTaskDone()
		assert.Nil(ji.Ts.T, err, "Error NTaskDone: %v", err)
		db.DPrintf(db.TEST, "[%v] ImgResizeJob NTaskDone: %v", ji.GetRealm(), n)
		if n == ji.ntasks {
			break
		}
		time.Sleep(1 * time.Second)
	}
	db.DPrintf(db.TEST, "[%v] Done waiting for ImgResizeJob to finish", ji.GetRealm())
	ji.imgd.StopGroup()
	db.DPrintf(db.TEST, "[%v] Imgd shutdown", ji.GetRealm())
}

func (ji *ImgResizeJobInstance) Cleanup() {
	dir := filepath.Join(sp.UX, sp.LOCAL, filepath.Dir(ji.input))
	db.DPrintf(db.TEST, "[%v] Cleaning up dir %v", ji.GetRealm(), dir)
	imgresize.Cleanup(ji.FsLib, dir)
}
