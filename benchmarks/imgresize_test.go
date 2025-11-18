package benchmarks_test

import (
	"path/filepath"

	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	"sigmaos/benchmarks"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
	rd "sigmaos/util/rand"
)

type ImgResizeJobInstance struct {
	sigmaos bool
	cfg     *benchmarks.ImgBenchConfig
	ready   chan bool

	runningTasks      chan bool
	sleepBetweenTasks time.Duration

	imgd   *imgresize.ImgdMgr[imgresize.Ttask]
	p      *perf.Perf
	ftclnt *imgresize.ImgdClnt[imgresize.Ttask]
	*test.RealmTstate
}

func NewImgResizeJob(ts *test.RealmTstate, p *perf.Perf, cfg *benchmarks.ImgBenchConfig, sigmaos bool) *ImgResizeJobInstance {
	ji := &ImgResizeJobInstance{}
	ji.sigmaos = sigmaos
	ji.cfg = cfg
	ji.cfg.JobCfg.Job = "imgresize-" + ts.GetRealm().String() + "-" + rd.String(4)
	ji.ready = make(chan bool)
	ji.RealmTstate = ts
	ji.p = p

	ji.sleepBetweenTasks = time.Second / time.Duration(ji.cfg.MaxRPS[0])
	ji.runningTasks = make(chan bool)

	ts.RmDir(sp.IMG)

	imgd, err := imgresize.NewImgdMgr[imgresize.Ttask](ji.SigmaClnt, ji.cfg.JobCfg, nil)
	assert.Nil(ji.Ts.T, err)
	ji.imgd = imgd

	ji.ftclnt, err = imgd.NewImgdClnt(ji.SigmaClnt)
	assert.Nil(ji.Ts.T, err)

	fn := ji.cfg.InputPath
	fns := make([]string, 0, ji.cfg.NInputsPerTask)
	for i := 0; i < ji.cfg.NInputsPerTask; i++ {
		fns = append(fns, fn)
	}

	db.DPrintf(db.ALWAYS, "Submit ImgResizeJob tasks")
	for i := 0; i < ji.cfg.NTasks; i++ {
		tasks := make([]*fttask_clnt.Task[imgresize.Ttask], 0, ji.cfg.NInputsPerTask)
		for _, fn := range fns {
			tasks = append(tasks, &fttask_clnt.Task[imgresize.Ttask]{
				Data: *imgresize.NewTask(fn, ji.cfg.JobCfg.UseS3Clnt, ji.cfg.JobCfg.UseBootScript),
			})
		}
		err := ji.ftclnt.SubmitTasks(tasks)
		assert.Nil(ji.Ts.T, err, "Error SubmitTask: %v", err)
	}
	// Sanity check
	n, err := ji.ftclnt.GetNTasks(fttask_clnt.TODO)
	assert.Nil(ji.Ts.T, err, "Error NTasksTODO: %v", err)
	assert.Equal(ji.Ts.T, n, ji.cfg.NTasks, "Num tasks TODO doesn't match ntasks")
	db.DPrintf(db.ALWAYS, "Done submitting ImgResize tasks")
	return ji
}

func (ji *ImgResizeJobInstance) StartImgResizeJob() {
	db.DPrintf(db.ALWAYS, "StartImgResizeJob input %v ntasks %v job %v", ji.cfg.InputPath, ji.cfg.NTasks, ji.cfg.JobCfg.Job)
}

func (ji *ImgResizeJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeJob to finish")
	for {
		n, err := ji.ftclnt.GetNTasks(fttask_clnt.TODO)
		assert.Nil(ji.Ts.T, err, "Error NTaskDone: %v", err)
		db.DPrintf(db.TEST, "[%v] ImgResizeJob NTaskDone: %v", ji.GetRealm(), n)
		if n == int32(ji.cfg.NTasks) {
			break
		}
		time.Sleep(1 * time.Second)
	}
	db.DPrintf(db.TEST, "[%v] Done waiting for ImgResizeJob to finish", ji.GetRealm())
	ji.imgd.StopImgd(true)
	db.DPrintf(db.TEST, "[%v] Imgd shutdown", ji.GetRealm())
}

func (ji *ImgResizeJobInstance) Cleanup() {
	dir := filepath.Join(sp.UX, sp.LOCAL, filepath.Dir(ji.cfg.InputPath))
	db.DPrintf(db.TEST, "[%v] Cleaning up dir %v", ji.GetRealm(), dir)
	imgresize.Cleanup(ji.FsLib, dir)
}
