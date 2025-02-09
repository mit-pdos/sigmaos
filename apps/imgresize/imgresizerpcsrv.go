package imgresize

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"sigmaos/api/fs"
	"sigmaos/apps/imgresize/proto"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttaskmgr "sigmaos/ft/task/mgr"
	fttask_srv "sigmaos/ft/task/srv"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type ImgSrvRPC struct {
	ftclnt     *fttask_clnt.FtTaskClnt[Ttask, any]
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	ssrv       *sigmasrv.SigmaSrv
	ndone      atomic.Int64
	mkProc     fttaskmgr.NewTmkProc[Ttask]
	sc         *sigmaclnt.SigmaClnt
}

func NewImgSrvRPC(args []string) (*ImgSrvRPC, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrvRPC{}

	serverId := args[0]
	ssrv, err := sigmasrv.NewSigmaSrv(filepath.Join(sp.IMG, serverId), imgd, proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaSrv: %v", err)
		return nil, err
	}
	imgd.sc = ssrv.SigmaClnt()
	imgd.ssrv = ssrv

	imgd.ftclnt = fttask_clnt.NewFtTaskClnt[Ttask, any](imgd.sc.FsLib, fttask_srv.FtTaskSrvId(serverId))
	db.DPrintf(db.IMGD, "Made imgd connected to %v", serverId)
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
	imgd.mkProc = getMkProcFn(imgd.ftclnt.ServerId, imgd.nrounds, imgd.workerMcpu, imgd.workerMem)
	return imgd, nil
}

func (imgd *ImgSrvRPC) Status(ctx fs.CtxI, req proto.StatusReq, rep *proto.StatusRep) error {
	rep.NDone = imgd.ndone.Load()
	return nil
}

func (imgd *ImgSrvRPC) Resize(ctx fs.CtxI, req proto.ImgResizeReq, rep *proto.ImgResizeRep) error {
	db.DPrintf(db.IMGD, "Resize %v", req)
	defer db.DPrintf(db.IMGD, "Resize %v done", req)

	t := NewTask(req.InputPath)
	p := imgd.mkProc(fttask_clnt.Task[Ttask]{
		Id: fttask_clnt.TaskId(0),
		Data: *t,
	})

	db.DPrintf(db.IMGD, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	// Spawn proc.
	err := imgd.sc.Spawn(p)
	if err != nil {
		db.DPrintf(db.IMGD_ERR, "Error spawn task: %v", err)
		db.DFatalf("Error spawn task: %v", err)
		rep.OK = false
		return nil
	}
	status, err := imgd.sc.WaitExit(p.GetPid())
	if err != nil || !status.IsStatusOK() {
		db.DPrintf(db.IMGD_ERR, "Error spawn task: %v", err)
		db.DFatalf("Error WaitExit task status %v err %v", status, err)
		rep.OK = false
		return nil
	}
	ms := time.Since(start).Milliseconds()
	imgd.ndone.Add(1)
	db.DPrintf(db.IMGD, "task ok [%v:%v] latency %vms", req.TaskName, p.GetPid(), ms)
	rep.OK = true
	return nil
}

func (imgd *ImgSrvRPC) Work() {
	db.DPrintf(db.IMGD, "Run imgd RPC server server %v", imgd.ftclnt.ServerId)
	if err := imgd.ssrv.RunServer(); err != nil {
		db.DFatalf("RunServer: %v", err)
	}
}
