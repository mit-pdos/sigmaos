// The clnt package provides a client for an imgresized server
package clnt

import (
	"fmt"
	"path/filepath"

	"sigmaos/apps/imgresize/proto"
	// db "sigmaos/debug"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type ImgResizeRPCClnt struct {
	job      string
	pn       string
	rpcclntc *rpcclnt.ClntCache
}

func NewImgResizeRPCClnt(fsl *fslib.FsLib, job string) (*ImgResizeRPCClnt, error) {
	return &ImgResizeRPCClnt{
		job:      job,
		pn:       filepath.Join(sp.IMG, job),
		rpcclntc: rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
	}, nil
}

func (clnt *ImgResizeRPCClnt) Resize(tname, ipath string) error {
	arg := proto.ImgResizeReq{
		TaskName:  tname,
		InputPath: ipath,
	}
	res := proto.ImgResizeRep{}
	err := clnt.rpcclntc.RPCRetryNotFound(clnt.pn, clnt.job, "ImgSrvRPC.Resize", &arg, &res)
	if err != nil {
		return err
	}
	if !res.OK {
		return fmt.Errorf("Resize error")
	}
	return nil
}

func (clnt *ImgResizeRPCClnt) Status() (int64, error) {
	arg := proto.StatusReq{}
	res := proto.StatusRep{}
	err := clnt.rpcclntc.RPCRetryNotFound(clnt.pn, clnt.job, "ImgSrvRPC.Status", &arg, &res)
	if err != nil {
		return 0, err
	}
	return res.NDone, nil
}

func (clnt *ImgResizeRPCClnt) ImgdFence() (sp.Tfence, error) {
	arg := proto.ImgFenceReq{}
	res := proto.ImgFenceRep{}
	err := clnt.rpcclntc.RPCRetryNotFound(clnt.pn, clnt.job, "ImgSrvRPC.ImgdFence", &arg, &res)
	if err != nil {
		return sp.NoFence(), err
	}
	return res.ImgdFence.Tfence(), nil
}
