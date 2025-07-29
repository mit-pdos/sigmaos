package srv

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_mgr "sigmaos/ft/task/fttaskmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/spstats"
)

type AStat struct {
	Nok    spstats.Tcounter
	Nerror spstats.Tcounter
	Nfail  spstats.Tcounter
}

type ImgSrv struct {
	sc         *sigmaclnt.SigmaClnt
	ftclnt     fttask_clnt.FtTaskClnt[imgresize.Ttask, any]
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	leaderclnt *leaderclnt.LeaderClnt
	imgSvcId   string
	taskSvcId  task.FtTaskSvcId
	ch         chan error
	AStat
}

func NewImgSrv(args []string) (*ImgSrv, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrv{}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	imgd.sc = sc

	imgd.imgSvcId = args[0]
	imgd.taskSvcId = task.FtTaskSvcId(args[4])
	db.DPrintf(db.IMGD, "Made imgd %v connected to task %v", imgd.imgSvcId, imgd.taskSvcId)

	mcpu, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse MCPU %v", err)
	}
	imgd.workerMcpu = proc.Tmcpu(mcpu)
	mem, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse Mem %v", err)
	}
	imgd.workerMem = proc.Tmem(mem)
	imgd.nrounds, err = strconv.Atoi(args[3])
	if err != nil {
		db.DFatalf("Error parse nrounds: %v", err)
	}

	imgd.sc.Started()

	pn := filepath.Join(sp.IMG, imgd.imgSvcId) + "-leader"
	imgd.leaderclnt, err = leaderclnt.NewLeaderClnt(imgd.sc.FsLib, pn, 0777)
	if err != nil {
		return nil, fmt.Errorf("NewLeaderclnt err %v", err)
	}

	imgd.ch = make(chan error)
	go imgd.sc.WaitExitChan(imgd.ch)

	return imgd, nil
}

func (imgd *ImgSrv) Work() {
	db.DPrintf(db.IMGD, "Try acquire leadership coord %v server %v", imgd.sc.ProcEnv().GetPID(), imgd.imgSvcId)

	// Try to become the leading coordinator.
	if err := imgd.leaderclnt.LeadAndFence(nil, []string{filepath.Join(sp.IMG, imgd.imgSvcId)}); err != nil {
		sts, _, err2 := imgd.sc.ReadDir(filepath.Join(sp.IMG, imgd.imgSvcId))
		db.DFatalf("LeadAndFence err %v sts %v err2 %v", err, sp.Names(sts), err2)
	}
	fence := imgd.leaderclnt.Fence()

	imgd.ftclnt = fttask_clnt.NewFtTaskClnt[imgresize.Ttask, any](imgd.sc.FsLib, imgd.taskSvcId, &fence)

	err := imgd.ftclnt.Fence(&fence)
	if err != nil {
		db.DFatalf("FtTaskClnt.Fence err %v", err)
	}

	db.DPrintf(db.FTTASKCLNT, "leader %s sigmafail %q fence %v", imgd.imgSvcId, proc.GetSigmaFail(), &fence)

	rpcs := NewRPCSrv(imgd)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.IMG, imgd.imgSvcId),
		imgd.sc, rpcs) // sesssrv.WithExp(imgd))
	if err != nil {
		db.DFatalf("NewSigmaSrvClnt: err %v", err)
	}

	go func() {
		<-imgd.ch
		ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
		os.Exit(0)
	}()

	db.DPrintf(db.FTTASKSRV, "Created imgd srv %s %v", imgd.imgSvcId, fence)

	if _, err := imgd.ftclnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO); err != nil {
		db.DFatalf("MoveTasksByStatus err %v", err)
	}

	ch := make(chan fttask_mgr.Tresult[imgresize.Ttask, any])
	ftc, err := fttask_mgr.NewFtTaskCoord(imgd.sc.ProcAPI, imgd.ftclnt, ch)
	if err != nil {
		db.DFatalf("NewFtTaskCoord err %v", err)
	}

	go imgd.processResults(ch)

	ftc.ExecuteTasks(imgresize.GetMkProcFn(imgd.ftclnt.ServiceId(), imgd.nrounds, imgd.workerMcpu, imgd.workerMem))
	close(ch)

	st := spstats.NewTcounterSnapshot()
	st.FillCounters(&imgd.AStat)

	//ids, err := ftc.GetTasksByStatus(fttask_clnt.ERROR)
	//if err != nil {
	//db.DFatalf("GetTasksByStatus err %v", err)
	//}

	db.DPrintf(db.ALWAYS, "imgresized exit %v", st)

	ssrv.SrvExit(proc.NewStatusInfo(proc.StatusOK, "OK", st))
}

func (imgd *ImgSrv) processResults(ch <-chan fttask_mgr.Tresult[imgresize.Ttask, any]) {
	for res := range ch {
		if res.Err == nil && res.Status.IsStatusOK() {
			spstats.Inc(&imgd.AStat.Nok, 1)
			if err := imgd.ftclnt.MoveTasks([]fttask_clnt.TaskId{res.Id}, fttask_clnt.DONE); err != nil {
				db.DFatalf("MoveTasks %v done err %v", res.Id, err)
			}
		} else if res.Err == nil && res.Status.IsStatusErr() && !res.Status.IsCrashed() {
			db.DPrintf(db.ALWAYS, "task %v errored status %v msg %v", res.Id, res.Status, res.Status.Msg())
			spstats.Inc(&imgd.AStat.Nerror, 1)
			// mark task as done, but return error
			if err := imgd.ftclnt.MoveTasks([]fttask_clnt.TaskId{res.Id}, fttask_clnt.ERROR); err != nil {
				db.DFatalf("MoveTasks %v error err %v", res.Id, err)
			}
			// XXX write status to output
			//if err := ftm.AddTaskOutputs([]ftclnt.TaskId{id}, status, false); err != nil {
			//	db.DFatalf("AddTaskOutputs %v error err %v", id, err)
			//}
		} else { // an error, task crashed, or was evicted; make it runnable again
			db.DPrintf(db.FTTASKMGR, "task %v failed %v err %v msg %q", res.Id, res.Status, res.Err, res.Status.Msg())
			spstats.Inc(&imgd.AStat.Nfail, 1)
			if err := imgd.ftclnt.MoveTasks([]fttask_clnt.TaskId{res.Id}, fttask_clnt.TODO); err != nil {
				db.DFatalf("MoveTasks %v todo err %v", res.Id, err)
			}
		}
	}
}
