package benchmarks_test

import (
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	rd "sigmaos/util/rand"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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
	srvProc           *proc.Proc
	rpcc              *imgresize.ImgResizeRPCClnt
	p                 *perf.Perf
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

	ts.RmDir(imgresize.IMG)

	if err := ji.MkDir(imgresize.IMG, 0777); err != nil {
		assert.True(ji.Ts.T, serr.IsErrCode(err, serr.TErrExists), "Unexpected err mkdir: %v", err)
	}

	return ji
}

func (ji *ImgResizeRPCJobInstance) runTask(wg *sync.WaitGroup, idx int) {
	defer wg.Done()

	err := ji.rpcc.Resize(strconv.Itoa(idx), ji.input)
	assert.Nil(ji.Ts.T, err, "Err Resize: %v", err)
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
	p, err := imgresize.StartImgRPCd(ji.SigmaClnt, ji.job, ji.mcpu, ji.mem, ji.nrounds, ji.imgdmcpu)
	assert.Nil(ji.Ts.T, err, "StartImgRPCd: %v", err)
	ji.srvProc = p
	rpcc, err := imgresize.NewImgResizeRPCClnt(ji.SigmaClnt.FsLib, ji.job)
	assert.Nil(ji.Ts.T, err)
	ji.rpcc = rpcc
	go ji.runTasks()
	db.DPrintf(db.ALWAYS, "Done starting ImgResizeRPC server")
}

func (ji *ImgResizeRPCJobInstance) Wait() {
	db.DPrintf(db.TEST, "Waiting for ImgResizeRPCJob to finish")
	<-ji.runningTasks
	ndone, err := ji.rpcc.Status()
	assert.Nil(ji.Ts.T, err, "Status: %v", err)
	db.DPrintf(db.TEST, "[%v] Done waiting for ImgResizeRPCJob to finish. Completed %v tasks", ji.GetRealm(), ndone)
	err = ji.Evict(ji.srvProc.GetPid())
	assert.Nil(ji.Ts.T, err)
	status, err := ji.WaitExit(ji.srvProc.GetPid())
	if assert.Nil(ji.Ts.T, err) {
		assert.True(ji.Ts.T, status.IsStatusEvicted(), "Wrong status: %v", status)
	}
	db.DPrintf(db.TEST, "[%v] Imgd shutdown", ji.GetRealm())
}

func (ji *ImgResizeRPCJobInstance) Cleanup() {
	dir := filepath.Join(sp.UX, sp.LOCAL, filepath.Dir(ji.input))
	db.DPrintf(db.TEST, "[%v] Cleaning up dir %v", ji.GetRealm(), dir)
	imgresize.Cleanup(ji.FsLib, dir)
}
