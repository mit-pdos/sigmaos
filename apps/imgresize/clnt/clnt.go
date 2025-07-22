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
	pn       string
	rpcclntc *rpcclnt.ClntCache
}

func NewImgResizeRPCClnt(fsl *fslib.FsLib, job string) (*ImgResizeRPCClnt, error) {
	return &ImgResizeRPCClnt{
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
	err := clnt.rpcclntc.RPC(clnt.pn, "ImgSrvRPC.Resize", &arg, &res)
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
	err := clnt.rpcclntc.RPC(clnt.pn, "ImgSrvRPC.Status", &arg, &res)
	if err != nil {
		return 0, err
	}
	return res.NDone, nil
}
