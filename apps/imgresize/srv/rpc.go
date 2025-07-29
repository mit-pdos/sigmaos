package srv

import (
	"sync/atomic"
	"time"

	"sigmaos/api/fs"
	"sigmaos/apps/imgresize"
	"sigmaos/apps/imgresize/proto"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_mgr "sigmaos/ft/task/procmgr"
)

type ImgSrvRPC struct {
	imgd   *ImgSrv
	mkProc fttask_mgr.TnewProc[imgresize.Ttask]
	ndone  atomic.Int64
}

func NewRPCSrv(imgd *ImgSrv) *ImgSrvRPC {
	rpcs := &ImgSrvRPC{imgd: imgd}
	rpcs.mkProc = imgresize.GetMkProcFn(imgd.ftclnt.ServiceId(), imgd.nrounds, imgd.workerMcpu, imgd.workerMem)
	return rpcs
}

func (rpcs *ImgSrvRPC) Resize(ctx fs.CtxI, req proto.ImgResizeReq, rep *proto.ImgResizeRep) error {
	db.DPrintf(db.IMGD, "Resize %v", req)
	defer db.DPrintf(db.IMGD, "Resize %v done", req)

	t := imgresize.NewTask(req.InputPath)
	p, err := rpcs.mkProc(fttask_clnt.Task[imgresize.Ttask]{
		Id:   fttask_clnt.TaskId(0),
		Data: *t,
	})
	if err != nil {
		db.DFatalf("Resize:mkProc err %", err)
	}

	db.DPrintf(db.IMGD, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	// Spawn proc.
	if err := rpcs.imgd.sc.Spawn(p); err != nil {
		db.DPrintf(db.IMGD_ERR, "Error spawn task: %v", err)
		db.DFatalf("Error spawn task: %v", err)
		rep.OK = false
		return nil
	}
	status, err := rpcs.imgd.sc.WaitExit(p.GetPid())
	if err != nil || !status.IsStatusOK() {
		db.DPrintf(db.IMGD_ERR, "Error spawn task: %v", err)
		db.DFatalf("Error WaitExit task status %v err %v", status, err)
		rep.OK = false
		return nil
	}
	ms := time.Since(start).Milliseconds()
	rpcs.ndone.Add(1)
	db.DPrintf(db.IMGD, "task ok [%v:%v] latency %vms", req.TaskName, p.GetPid(), ms)
	rep.OK = true
	return nil
}

func (imgd *ImgSrvRPC) Status(ctx fs.CtxI, req proto.StatusReq, rep *proto.StatusRep) error {
	rep.NDone = imgd.ndone.Load()
	return nil
}

func (imgd *ImgSrvRPC) ImgdFence(ctx fs.CtxI, req proto.ImgFenceReq, rep *proto.ImgFenceRep) error {
	f := imgd.imgd.leaderclnt.Fence()
	rep.ImgdFence = f.FenceProto()
	return nil
}
