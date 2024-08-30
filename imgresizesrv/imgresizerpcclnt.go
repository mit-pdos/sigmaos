package imgresizesrv

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/imgresizesrv/proto"
	"sigmaos/rpcclnt"
	"sigmaos/sigmarpcchan"
)

type ImgResizeRPCClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewImgResizeRPCClnt(fsl *fslib.FsLib, job string) (*ImgResizeRPCClnt, error) {
	rpcc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{fsl}, filepath.Join(IMG, job))
	if err != nil {
		db.DPrintf(db.ERROR, "NewSigmaRPCClnt: %v", err)
		return nil, err
	}
	return &ImgResizeRPCClnt{
		rpcc: rpcc,
	}, nil
}

func (clnt *ImgResizeRPCClnt) Resize(tname, ipath string) error {
	arg := proto.ImgResizeRequest{
		TaskName:  tname,
		InputPath: ipath,
	}
	res := proto.ImgResizeResult{}
	err := clnt.rpcc.RPC("ImgSrvRPC.Resize", &arg, &res)
	if err != nil {
		return err
	}
	if !res.OK {
		return fmt.Errorf("Err res not OK")
	}
	return nil
}
