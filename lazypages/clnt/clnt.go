package clnt

import (
	"sigmaos/fslib"
	"sigmaos/lazypages"
	"sigmaos/lazypages/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	sp "sigmaos/sigmap"
)

type LazyPagesClnt struct {
	*rpcclnt.RPCClnt
	wd string
}

func NewLazyPagesClnt(fsl *fslib.FsLib, pid sp.Tpid) (*LazyPagesClnt, error) {
	pn := lazypages.SrvPath(pid)
	wd := lazypages.WorkDir(pid)
	rc, err := sprpcclnt.NewRPCClnt(fsl, pn)
	if err != nil {
		return nil, err
	}
	lpc := &LazyPagesClnt{rc, wd}
	return lpc, nil
}

func (lpc *LazyPagesClnt) WorkDir() string {
	return lpc.wd
}

func (lpc *LazyPagesClnt) Register(pid int, imgdir string, pages string, ckptDir string, firstInstance bool) error {
	req := &proto.RegisterRequest{
		Pid:           uint32(pid),
		ImgDir:        imgdir,
		Pages:         pages,
		CkptDir:       ckptDir,
		FirstInstance: firstInstance,
	}
	res := &proto.RegisterResult{}
	if err := lpc.RPC("LazyPagesSvc.Register", req, res); err != nil {
		return err
	}
	return nil
}
