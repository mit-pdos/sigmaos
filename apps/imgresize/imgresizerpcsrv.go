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
	fttaskmgr "sigmaos/ft/task/mgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type ImgSrvRPC struct {
	job        string
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	ssrv       *sigmasrv.SigmaSrv
	ndone      atomic.Int64
	mkProc     fttaskmgr.TmkProc
	sc         *sigmaclnt.SigmaClnt
}

func NewImgSrvRPC(args []string) (*ImgSrvRPC, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrvRPC{}
	imgd.job = args[0]

	ssrv, err := sigmasrv.NewSigmaSrv(filepath.Join(sp.IMG, imgd.job), imgd, proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaSrv: %v", err)
		return nil, err
	}
	imgd.sc = ssrv.SigmaClnt()
	imgd.ssrv = ssrv
	db.DPrintf(db.IMGD, "Made fslib job %v", imgd.job)
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
	imgd.mkProc = getMkProcFn(imgd.job, imgd.nrounds, imgd.workerMcpu, imgd.workerMem)
	return imgd, nil
}

func (imgd *ImgSrvRPC) Status(ctx fs.CtxI, req proto.StatusRequest, rep *proto.StatusResult) error {
	rep.NDone = imgd.ndone.Load()
	return nil
}

func (imgd *ImgSrvRPC) Resize(ctx fs.CtxI, req proto.ImgResizeRequest, rep *proto.ImgResizeResult) error {
	db.DPrintf(db.IMGD, "Resize %v", req)
	defer db.DPrintf(db.IMGD, "Resize %v done", req)

	t := NewTask(req.InputPath)
	p := imgd.mkProc(req.TaskName, t)

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
	db.DPrintf(db.IMGD, "Run imgd RPC server job %v", imgd.job)
	if err := imgd.ssrv.RunServer(); err != nil {
		db.DFatalf("RunServer: %v", err)
	}
}
