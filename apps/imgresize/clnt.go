package imgresize

import (
	rpcclnt "sigmaos/apps/imgresize/rpcclnt"
	db "sigmaos/debug"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/sigmaclnt"
	//sp "sigmaos/sigmap"
)

// A client for imgresized service, providing both an RPC interface
// for the service and the fttask clnt interface.  The RPC interface
// calls don't persist their work.

type ImgdClnt[Data any] struct {
	rpcc   *rpcclnt.ImgResizeRPCClnt
	ftclnt fttask_clnt.FtTaskClnt[Data, any]
}

func NewImgdClnt[Data any](sc *sigmaclnt.SigmaClnt, job string, id task.FtTaskSvcId, a *fttask_clnt.AcquireId) (*ImgdClnt[Data], error) {
	rpcc, err := rpcclnt.NewImgResizeRPCClnt(sc.FsLib, ImgSvcId(job))
	if err != nil {
		return nil, err
	}
	return &ImgdClnt[Data]{
		rpcc:   rpcc,
		ftclnt: fttask_clnt.NewFtTaskClnt[Data, any](sc.FsLib, id, a),
	}, nil
}

func (clnt *ImgdClnt[Data]) SubmitTasks(tasks []*fttask_clnt.Task[Data]) error {
	return clnt.ftclnt.SubmitTasks(tasks)
}

func (clnt *ImgdClnt[Data]) SubmittedLastTask() error {
	return clnt.ftclnt.SubmittedLastTask()
}

func (clnt *ImgdClnt[Data]) GetNTasks(status fttask_clnt.TaskStatus) (int32, error) {
	return clnt.ftclnt.GetNTasks(status)
}

func (clnt *ImgdClnt[Data]) Resize(tname, ipath string) error {
	return clnt.rpcc.Resize(tname, ipath)
}

func (clnt *ImgdClnt[Data]) Status() (int64, error) {
	return clnt.rpcc.Status()
}

func (clnt *ImgdClnt[Data]) SetImgdFence() error {
	f, err := clnt.rpcc.ImgdFence()
	db.DPrintf(db.IMGD, "SetImgdFence: %v err %v", f, err)
	if err != nil {
		return nil
	}
	clnt.ftclnt.SetFence(&f)
	return nil
}
