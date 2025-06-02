package clnt

import (
	"fmt"
	"path/filepath"

	"sigmaos/apps/cossim"
	"sigmaos/apps/cossim/proto"
	db "sigmaos/debug"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type CosSimClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewCosSimClnt(fsl *fslib.FsLib) (*CosSimClnt, error) {
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, cossim.COSSIM)
	if err != nil {
		return nil, err
	}
	return &CosSimClnt{
		fsl:  fsl,
		rpcc: rpcc,
	}, nil
}

// Register a service's endpoint
func (clnt *CosSimClnt) CosSim(b []byte, n int64) (uint64, error) {
	db.DPrintf(db.COSSIMCLNT, "CosSim: %v", len(b))
	var res proto.CosSimRep
	req := &proto.CosSimReq{
		InputVec: b,
		N:        n,
	}
	err := clnt.rpcc.RPC("CosSimSrv.CosSim", req, &res)
	if err != nil {
		db.DPrintf(db.COSSIMCLNT_ERR, "Err Register: %v", err)
		return 0, err
	}
	db.DPrintf(db.COSSIMCLNT, "CosSim ok: %v -> %v", len(b), res.ID)
	return res.ID, nil
}
