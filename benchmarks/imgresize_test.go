package benchmarks_test

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_srv "sigmaos/ft/task/srv"
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
	imgd     *procgroupmgr.ProcGroupMgr
	p        *perf.Perf
	ftmgr    *fttask_srv.FtTaskSrvMgr
	ftclnt   fttask_clnt.FtTaskClnt[imgresize.Ttask, any]
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

	ts.RmDir(sp.IMG)

	ftid := fmt.Sprintf("imgresize-%s", ji.job)
	ftmgr, err := fttask_srv.NewFtTaskSrvMgr(ji.SigmaClnt, ftid, false)
	assert.Nil(ts.Ts.T, err, "Error new fttasksrvmgr: %v", err)

	ji.ftmgr = ftmgr
	ji.ftclnt = fttask_clnt.NewFtTaskClnt[imgresize.Ttask, any](ji.SigmaClnt.FsLib, ftmgr.Id)

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
				Data: *imgresize.NewTask(fn),
			})
		}
		existing, err := ji.ftclnt.SubmitTasks(tasks)
		assert.Empty(ji.Ts.T, existing)
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
	ji.imgd = imgresize.StartImgd(ji.SigmaClnt, imgresize.ImgSvcId(ji.job), ji.ftclnt.ServiceId().String(), ji.mcpu, ji.mem, false, ji.nrounds, ji.imgdmcpu, nil)
	db.DPrintf(db.ALWAYS, "Done starting ImgResizeJob")
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
	ji.imgd.StopGroup()
	db.DPrintf(db.TEST, "[%v] Imgd shutdown", ji.GetRealm())
}

func (ji *ImgResizeJobInstance) Cleanup() {
	dir := filepath.Join(sp.UX, sp.LOCAL, filepath.Dir(ji.input))
	db.DPrintf(db.TEST, "[%v] Cleaning up dir %v", ji.GetRealm(), dir)
	imgresize.Cleanup(ji.FsLib, dir)
}
