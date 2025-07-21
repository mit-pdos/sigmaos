package clnt

import (
	"fmt"
	"path/filepath"

	"sigmaos/apps/imgresize/proto"
	db "sigmaos/debug"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type ImgResizeRPCClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewImgResizeRPCClnt(fsl *fslib.FsLib, job string) (*ImgResizeRPCClnt, error) {
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, filepath.Join(sp.IMG, job))
	if err != nil {
		db.DPrintf(db.ERROR, "NewSigmaRPCClnt: %v", err)
		return nil, err
	}
	return &ImgResizeRPCClnt{
		rpcc: rpcc,
	}, nil
}

func (clnt *ImgResizeRPCClnt) Resize(tname, ipath string) error {
	arg := proto.ImgResizeReq{
		TaskName:  tname,
		InputPath: ipath,
	}
	res := proto.ImgResizeRep{}
	err := clnt.rpcc.RPC("ImgSrvRPC.Resize", &arg, &res)
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
	err := clnt.rpcc.RPC("ImgSrvRPC.Status", &arg, &res)
	if err != nil {
		return 0, err
	}
	return res.NDone, nil
}
