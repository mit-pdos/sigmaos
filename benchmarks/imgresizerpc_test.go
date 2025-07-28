package benchmarks_test

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	//	imgd_clnt "sigmaos/apps/imgresize/clnt"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
	rd "sigmaos/util/rand"
)

type ImgResizeRPCJobInstance struct {
	sigmaos           bool
	job               string
	imgdmcpu          proc.Tmcpu
	mcpu              proc.Tmcpu
	mem               proc.Tmem
	tasksPerSecond    int
	dur               time.Duration
	nrounds           int
	input             string
	runningTasks      chan bool
	ready             chan bool
	sleepBetweenTasks time.Duration
	pgm               *procgroupmgr.ProcGroupMgr
	//	rpcc              *imgd_clnt.ImgResizeRPCClnt
	p *perf.Perf
	*test.RealmTstate
}

func NewImgResizeRPCJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, tasksPerSecond int, dur time.Duration, mcpu proc.Tmcpu, mem proc.Tmem, nrounds int, imgdmcpu proc.Tmcpu) *ImgResizeRPCJobInstance {
	ji := &ImgResizeRPCJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = "imgresize-" + ts.GetRealm().String() + "-" + rd.String(4)
	ji.tasksPerSecond = tasksPerSecond
	ji.dur = dur
	ji.input = input
	ji.runningTasks = make(chan bool)
	ji.ready = make(chan bool)
	ji.RealmTstate = ts
	ji.p = p
	ji.mcpu = mcpu
	ji.imgdmcpu = imgdmcpu
	ji.mem = mem
	ji.nrounds = nrounds
	ji.sleepBetweenTasks = time.Second / time.Duration(ji.tasksPerSecond)

	ts.RmDir(sp.IMG)

	if err := ji.MkDir(sp.IMG, 0777); err != nil {
		assert.True(ji.Ts.T, serr.IsErrCode(err, serr.TErrExists), "Unexpected err mkdir: %v", err)
	}

	return ji
}

func (ji *ImgResizeRPCJobInstance) runTask(wg *sync.WaitGroup, idx int) {
	defer wg.Done()

	// err := ji.rpcc.Resize(strconv.Itoa(idx), ji.input)
	// assert.Nil(ji.Ts.T, err, "Err Resize: %v", err)
}

func (ji *ImgResizeRPCJobInstance) runTasks() {
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; time.Since(start) < ji.dur; i++ {
		wg.Add(1)
		go ji.runTask(&wg, i)
		time.Sleep(ji.sleepBetweenTasks)
	}
	wg.Wait()
	ji.runningTasks <- false
}

func (ji *ImgResizeRPCJobInstance) StartImgResizeRPCJob() {
	db.DPrintf(db.ALWAYS, "StartImgResizeRPC server input %v tps %v dur %v mcpu %v job %v", ji.input, ji.tasksPerSecond, ji.dur, ji.mcpu, ji.job)
	//	ji.pgm = imgresize.StartImgd(ji.SigmaClnt, ji.job, "", ji.mcpu, ji.mem, false, ji.nrounds, ji.imgdmcpu, nil)
	//	rpcc, err := imgd_clnt.NewImgResizeRPCClnt(ji.SigmaClnt.FsLib, ji.job)
	//	assert.Nil(ji.Ts.T, err)
	//	ji.rpcc = rpcc
	go ji.runTasks()
	db.DPrintf(db.ALWAYS, "Done starting ImgResizeRPC server")
}

func (ji *ImgResizeRPCJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeRPCJob to finish")
	<-ji.runningTasks
	//	ndone, err := ji.rpcc.Status()
	//	assert.Nil(ji.Ts.T, err, "Status: %v", err)
	//	db.DPrintf(db.TEST, "[%v] Done waiting for ImgResizeRPCJob to finish. Completed %v tasks", ji.GetRealm(), ndone)
	//	sts, err := ji.pgm.StopGroup()
	//	if assert.Nil(ji.Ts.T, err) {
	//		assert.True(ji.Ts.T, sts[0].IsStatusEvicted(), "Wrong status: %v", sts[0])
	//	}
	//	db.DPrintf(db.TEST, "[%v] Imgd shutdown", ji.GetRealm())
}

func (ji *ImgResizeRPCJobInstance) Cleanup() {
	dir := filepath.Join(sp.UX, sp.LOCAL, filepath.Dir(ji.input))
	db.DPrintf(db.TEST, "[%v] Cleaning up dir %v", ji.GetRealm(), dir)
	imgresize.Cleanup(ji.FsLib, dir)
}
