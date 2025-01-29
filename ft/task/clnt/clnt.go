package clnt

import (
	"path/filepath"
	"sigmaos/ft/task/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type TaskClnt struct {
	*rpcclnt.ClntCache
	fsl         *fslib.FsLib
	serverPath  string
}

func NewTaskClnt(fsl *fslib.FsLib, serverId string) *TaskClnt {
	tc := &TaskClnt{
		fsl:       fsl,
		ClntCache: rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
		serverPath: filepath.Join(sp.NAMED, "fttask", serverId),
	}
	return tc
}

func (tc *TaskClnt) Echo(text string) (string, error) {
	arg := proto.EchoReq{Text: text}
	res := proto.EchoRep{}

	err := tc.RPC(tc.serverPath, "TaskSrv.Echo", &arg, &res)
	return res.Text, err
}