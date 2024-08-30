package lazypagesclnt

import (
	"sigmaos/fslib"
	"sigmaos/lazypagessrv"
	"sigmaos/lazypagessrv/proto"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
)

type LazyPagesClnt struct {
	*rpcclnt.RPCClnt
	wd string
}

func NewLazyPagesClnt(fsl *fslib.FsLib, pid sp.Tpid) (*LazyPagesClnt, error) {
	pn := lazypagessrv.SrvPath(pid)
	wd := lazypagessrv.WorkDir(pid)
	rc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{fsl}, pn)
	if err != nil {
		return nil, err
	}
	lpc := &LazyPagesClnt{rc, wd}
	return lpc, nil
}

func (lpc *LazyPagesClnt) WorkDir() string {
	return lpc.wd
}

func (lpc *LazyPagesClnt) Register(imgdir string, pages string) error {
	req := &proto.RegisterRequest{
		ImgDir: imgdir,
		Pages:  pages,
	}
	res := &proto.RegisterResult{}
	if err := lpc.RPC("LazyPagesSvc.Register", req, res); err != nil {
		return err
	}
	return nil
}
