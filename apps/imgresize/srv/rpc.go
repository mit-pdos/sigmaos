package srv

import (
	"sync/atomic"
	"time"

	"sigmaos/api/fs"
	"sigmaos/apps/imgresize"
	"sigmaos/apps/imgresize/proto"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_coord "sigmaos/ft/task/coord"
)

type RPCsrv struct {
	imgd   *ImgSrv
	mkProc fttask_coord.TmkProc[imgresize.Ttask]
	ndone  atomic.Int64
}

func NewRPCSrv(imgd *ImgSrv) *RPCsrv {
	rpcs := &RPCsrv{imgd: imgd}
	rpcs.mkProc = imgresize.GetMkProcFn(imgd.ftclnt.ServiceId(), imgd.nrounds, imgd.workerMcpu, imgd.workerMem)
	return rpcs
}

func (rpcs *RPCsrv) Resize(ctx fs.CtxI, req proto.ImgResizeReq, rep *proto.ImgResizeRep) error {
	db.DPrintf(db.IMGD, "Resize %v", req)
	defer db.DPrintf(db.IMGD, "Resize %v done", req)

	t := imgresize.NewTask(req.InputPath)
	p := rpcs.mkProc(fttask_clnt.Task[imgresize.Ttask]{
		Id:   fttask_clnt.TaskId(0),
		Data: *t,
	})

	db.DPrintf(db.IMGD, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	// Spawn proc.
	err := rpcs.imgd.sc.Spawn(p)
	if err != nil {
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
