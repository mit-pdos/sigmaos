package imgresize

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	// "sigmaos/util/crash"
	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_coord "sigmaos/ft/task/coord"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type ImgSrv struct {
	*sigmaclnt.SigmaClnt
	ftclnt     fttask_clnt.FtTaskClnt[Ttask, any]
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	exited     bool
	leaderclnt *leaderclnt.LeaderClnt
}

func NewImgSrv(args []string) (*ImgSrv, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrv{}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	imgd.SigmaClnt = sc

	serverId := args[0]
	db.DPrintf(db.IMGD, "Made imgd connected to %v", serverId)

	imgd.ftclnt = fttask_clnt.NewFtTaskClnt[Ttask, any](sc.FsLib, task.FtTaskSrvId(serverId))
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

	imgd.Started()

	folder := filepath.Join(sp.IMG, string(imgd.ftclnt.ServerId()))
	imgd.leaderclnt, err = leaderclnt.NewLeaderClnt(imgd.FsLib, filepath.Join(folder, "imgd-leader"), 0777)
	if err != nil {
		return nil, fmt.Errorf("NewLeaderclnt err %v", err)
	}

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
	db.DPrintf(db.IMGD, "Try acquire leadership coord %v server %v", imgd.ProcEnv().GetPID(), imgd.ftclnt.ServerId())

	if err := imgd.MkDirPath(sp.NAMED, filepath.Join(sp.IMGREL, string(imgd.ftclnt.ServerId())), 0777); err != nil && !serr.IsErrorExists(err) {
		db.DFatalf("MkDirPath err %v", err)
	}

	// Try to become the leading coordinator.
	if err := imgd.leaderclnt.LeadAndFence(nil, []string{filepath.Join(sp.IMG, string(imgd.ftclnt.ServerId()))}); err != nil {
		sts, _, err2 := imgd.ReadDir(filepath.Join(sp.IMG, string(imgd.ftclnt.ServerId())))
		db.DFatalf("LeadAndFence err %v sts %v err2 %v", err, sp.Names(sts), err2)
	}

	fence := imgd.leaderclnt.Fence()
	err := imgd.ftclnt.Fence(&fence)
	if err != nil {
		db.DFatalf("FtTaskClnt.Fence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s fail %q", imgd.ftclnt.ServerId(), proc.GetSigmaFail())

	ftm, err := fttask_coord.NewFtTaskCoord(imgd.SigmaClnt.ProcAPI, imgd.ftclnt)
	if err != nil {
		db.DFatalf("NewTaskMgr err %v", err)
	}
	status := ftm.ExecuteTasks(getMkProcFn(imgd.ftclnt.ServerId(), imgd.nrounds, imgd.workerMcpu, imgd.workerMem))
	db.DPrintf(db.ALWAYS, "imgresized exit")
	imgd.exited = true
	if status == nil {
		imgd.ClntExitOK()
	} else {
		imgd.ClntExit(proc.NewStatusInfo(proc.StatusFatal, "task error", status))
	}
}
