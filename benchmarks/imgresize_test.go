package benchmarks_test

import (
	"path/filepath"

	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
	rd "sigmaos/util/rand"
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

	runningTasks      chan bool
	sleepBetweenTasks time.Duration
	tasksPerSecond    int
	dur               time.Duration

	imgd   *imgresize.ImgdMgr[imgresize.Ttask]
	p      *perf.Perf
	ftclnt *imgresize.ImgdClnt[imgresize.Ttask]
	*test.RealmTstate
}

func NewImgResizeJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, ntasks int, ninputs int, tasksPerSecond int, dur time.Duration, mcpu proc.Tmcpu, mem proc.Tmem, nrounds int, imgdmcpu proc.Tmcpu) *ImgResizeJobInstance {
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

	ji.tasksPerSecond = tasksPerSecond
	ji.dur = dur
	ji.sleepBetweenTasks = time.Second / time.Duration(ji.tasksPerSecond)
	ji.tasksPerSecond = tasksPerSecond
	ji.runningTasks = make(chan bool)

	ts.RmDir(sp.IMG)

	imgd, err := imgresize.NewImgdMgr[imgresize.Ttask](ji.SigmaClnt, imgresize.ImgSvcId(ji.job), ji.mcpu, ji.mem, false, ji.nrounds, ji.imgdmcpu, nil, false, false)
	assert.Nil(ji.Ts.T, err)
	ji.imgd = imgd

	ji.ftclnt, err = imgd.NewImgdClnt(ji.SigmaClnt)
	assert.Nil(ji.Ts.T, err)

	fn := ji.input
	fns := make([]string, 0, ji.ninputs)
	for i := 0; i < ji.ninputs; i++ {
		fns = append(fns, fn)
	}

	db.DPrintf(db.ALWAYS, "Submit ImgResizeJob tasks")
	for i := 0; i < ji.ntasks; i++ {
		tasks := make([]*fttask_clnt.Task[imgresize.Ttask], 0, ji.ninputs)
		for _, fn := range fns {
			tasks = append(tasks, &fttask_clnt.Task[imgresize.Ttask]{
				Data: *imgresize.NewTask(fn, false, false),
			})
		}
		err := ji.ftclnt.SubmitTasks(tasks)
		assert.Nil(ji.Ts.T, err, "Error SubmitTask: %v", err)
	}
	// Sanity check
	n, err := ji.ftclnt.GetNTasks(fttask_clnt.TODO)
	assert.Nil(ji.Ts.T, err, "Error NTasksTODO: %v", err)
	assert.Equal(ji.Ts.T, n, ji.ntasks, "Num tasks TODO doesn't match ntasks")
	db.DPrintf(db.ALWAYS, "Done submitting ImgResize tasks")
	return ji
}

func (ji *ImgResizeJobInstance) StartImgResizeJob() {
	db.DPrintf(db.ALWAYS, "StartImgResizeJob input %v ntasks %v mcpu %v job %v", ji.input, ji.ntasks, ji.mcpu, ji.job)
}

func (ji *ImgResizeJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeJob to finish")
	for {
		n, err := ji.ftclnt.GetNTasks(fttask_clnt.TODO)
		assert.Nil(ji.Ts.T, err, "Error NTaskDone: %v", err)
		db.DPrintf(db.TEST, "[%v] ImgResizeJob NTaskDone: %v", ji.GetRealm(), n)
		if n == int32(ji.ntasks) {
			break
		}
		time.Sleep(1 * time.Second)
	}
	db.DPrintf(db.TEST, "[%v] Done waiting for ImgResizeJob to finish", ji.GetRealm())
	ji.imgd.StopImgd(true)
	db.DPrintf(db.TEST, "[%v] Imgd shutdown", ji.GetRealm())
}

func (ji *ImgResizeJobInstance) Cleanup() {
	dir := filepath.Join(sp.UX, sp.LOCAL, filepath.Dir(ji.input))
	db.DPrintf(db.TEST, "[%v] Cleaning up dir %v", ji.GetRealm(), dir)
	imgresize.Cleanup(ji.FsLib, dir)
}
