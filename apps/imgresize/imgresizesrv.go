package imgresize

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"sigmaos/crash"
	db "sigmaos/debug"
	fttaskmgr "sigmaos/fttask/mgr"
	"sigmaos/fttask"
	"sigmaos/leaderclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type ImgSrv struct {
	*sigmaclnt.SigmaClnt
	ft         *fttask.FtTasks
	job        string
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	crash      int64
	exited     bool
	leaderclnt *leaderclnt.LeaderClnt
	stop       int32
}

func NewImgSrv(args []string) (*ImgSrv, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrv{}
	imgd.job = args[0]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.IMGD, "Made fslib job %v", imgd.job)
	imgd.SigmaClnt = sc
	crashing, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: error parse crash %v", err)
	}
	imgd.crash = int64(crashing)
	imgd.ft, err = fttask.NewFtTasks(sc.FsLib, IMG, imgd.job)
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: NewFtTasks %v", err)
	}
	mcpu, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse MCPU %v", err)
	}
	imgd.workerMcpu = proc.Tmcpu(mcpu)
	mem, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse Mem %v", err)
	}
	imgd.workerMem = proc.Tmem(mem)
	imgd.nrounds, err = strconv.Atoi(args[4])
	if err != nil {
		db.DFatalf("Error parse nrounds: %v", err)
	}

	imgd.Started()

	imgd.leaderclnt, err = leaderclnt.NewLeaderClnt(imgd.FsLib, filepath.Join(IMG, imgd.job, "imgd-leader"), 0777)
	if err != nil {
		return nil, fmt.Errorf("NewLeaderclnt err %v", err)
	}

	crash.Crasher(imgd.FsLib)

	go func() {
		imgd.WaitEvict(sc.ProcEnv().GetPID())
		if !imgd.exited {
			imgd.ClntExitOK()
		}
		os.Exit(0)
	}()

	return imgd, nil
}

func (imgd *ImgSrv) Work() {
	db.DPrintf(db.IMGD, "Try acquire leadership coord %v job %v", imgd.ProcEnv().GetPID(), imgd.job)

	// Try to become the leading coordinator.
	if err := imgd.leaderclnt.LeadAndFence(nil, []string{filepath.Join(IMG, imgd.job)}); err != nil {
		sts, err2 := imgd.ft.Jobs()
		db.DFatalf("LeadAndFence err %v sts %v err2 %v", err, sp.Names(sts), err2)
	}

	db.DPrintf(db.ALWAYS, "leader %s", imgd.job)

	ftm, err := fttaskmgr.NewTaskMgr(imgd.SigmaClnt.ProcAPI, imgd.ft)
	if err != nil {
		db.DFatalf("NewTaskMgr err %v", err)
	}
	status := ftm.ExecuteTasks(func() interface{} { return new(Ttask) }, getMkProcFn(imgd.job, imgd.nrounds, imgd.crash, imgd.workerMcpu, imgd.workerMem))
	db.DPrintf(db.ALWAYS, "imgresized exit")
	imgd.exited = true
	if status == nil {
		imgd.ClntExitOK()
	} else {
		imgd.ClntExit(proc.NewStatusInfo(proc.StatusFatal, "task error", status))
	}
}
