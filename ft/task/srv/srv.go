package srv

import (
	"path/filepath"
	"sigmaos/api/fs"
	"sigmaos/ft/task/proto"
	"sigmaos/proc"
	"sigmaos/sigmasrv"

	sp "sigmaos/sigmap"
)

type TaskSrv struct {

}

func RunTaskSrv(args []string) error {
	pe := proc.GetProcEnv()
	s := &TaskSrv{}

	id := args[0]

	ssrv, err := sigmasrv.NewSigmaSrv(filepath.Join(sp.NAMED, "fttask", id), s, pe)
	if err != nil {
		return err
	}

	return ssrv.RunServer()
}

func (s *TaskSrv) Echo(ctx fs.CtxI, req proto.EchoReq, rep *proto.EchoRep) error {
	rep.Text = req.Text
	return nil
}