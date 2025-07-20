package srv

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
	// sesssrv "sigmaos/session/srv"
	"sigmaos/apps/imgresize"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type ImgSrv struct {
	sc         *sigmaclnt.SigmaClnt
	ftclnt     fttask_clnt.FtTaskClnt[imgresize.Ttask, any]
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	leaderclnt *leaderclnt.LeaderClnt
	serviceId  string
	ch         chan error
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
	imgd.sc = sc

	imgd.serviceId = args[0]
	db.DPrintf(db.IMGD, "Made imgd connected to %v", imgd.serviceId)

	imgd.ftclnt = fttask_clnt.NewFtTaskClnt[imgresize.Ttask, any](sc.FsLib, task.FtTaskSvcId(imgd.serviceId))
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

	folder := filepath.Join(sp.IMG, string(imgd.ftclnt.ServiceId()))
	imgd.leaderclnt, err = leaderclnt.NewLeaderClnt(imgd.sc.FsLib, filepath.Join(folder, "imgd-leader"), 0777)
	if err != nil {
		return nil, fmt.Errorf("NewLeaderclnt err %v", err)
	}

	imgd.ch = make(chan error)
	go imgd.sc.WaitExitChan(imgd.ch)

	return imgd, nil
}

func (imgd *ImgSrv) Work() {
	db.DPrintf(db.IMGD, "Try acquire leadership coord %v server %v", imgd.sc.ProcEnv().GetPID(), imgd.ftclnt.ServiceId())

	if err := imgd.sc.MkDirPath(sp.NAMED, filepath.Join(sp.IMGREL, string(imgd.ftclnt.ServiceId())), 0777); err != nil && !serr.IsErrorExists(err) {
		db.DFatalf("MkDirPath err %v", err)
	}

	// Try to become the leading coordinator.
	if err := imgd.leaderclnt.LeadAndFence(nil, []string{filepath.Join(sp.IMG, string(imgd.ftclnt.ServiceId()))}); err != nil {
		sts, _, err2 := imgd.sc.ReadDir(filepath.Join(sp.IMG, string(imgd.ftclnt.ServiceId())))
		db.DFatalf("LeadAndFence err %v sts %v err2 %v", err, sp.Names(sts), err2)
	}

	fence := imgd.leaderclnt.Fence()
	err := imgd.ftclnt.Fence(&fence)
	if err != nil {
		db.DFatalf("FtTaskClnt.Fence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s sigmafail %q", imgd.ftclnt.ServiceId(), proc.GetSigmaFail())

	rpcs := NewRPCSrv(imgd)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.IMG, imgd.serviceId),
		imgd.sc, rpcs) // sesssrv.WithExp(imgd))
	if err != nil {
		db.DFatalf("NewSigmaSrvClnt: err %v", err)
	}

	go func() {
		<-imgd.ch
		ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
		os.Exit(0)
	}()

	db.DPrintf(db.FTTASKSRV, "Created imgd srv %s", imgd.serviceId)

	ftc, err := fttask_coord.NewFtTaskCoord(imgd.sc.ProcAPI, imgd.ftclnt)
	if err != nil {
		db.DFatalf("NewTaskMgr err %v", err)
	}
	st := ftc.ExecuteTasks(imgresize.GetMkProcFn(imgd.ftclnt.ServiceId(), imgd.nrounds, imgd.workerMcpu, imgd.workerMem))

	//ids, err := ftc.GetTasksByStatus(fttask_clnt.ERROR)
	//if err != nil {
	//db.DFatalf("GetTasksByStatus err %v", err)
	//}

	db.DPrintf(db.ALWAYS, "imgresized exit %v", st)

	ssrv.SrvExit(proc.NewStatusInfo(proc.StatusOK, "OK", st))
}
