package clnt

import (
	"fmt"

	"sigmaos/apps/cossim"
	"sigmaos/apps/cossim/proto"
	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	rpcclnt "sigmaos/rpc/clnt"
	rpcncclnt "sigmaos/rpc/clnt/netconn"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type CosSimClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
	epcc *epcacheclnt.EndpointCacheClnt
}

func NewCosSimClnt(fsl *fslib.FsLib, epcc *epcacheclnt.EndpointCacheClnt, srvID string) (*CosSimClnt, error) {
	instances, _, err := epcc.GetEndpoints(cossim.COSSIM, epcache.NO_VERSION)
	if err != nil {
		db.DPrintf(db.COSSIMCLNT_ERR, "Err GetEndpoints: %v", err)
		return nil, err
	}
	var ep *sp.Tendpoint
	for _, i := range instances {
		if i.ID == srvID {
			ep = sp.NewEndpointFromProto(i.EndpointProto)
		}
	}
	if ep == nil {
		db.DPrintf(db.COSSIMCLNT_ERR, "Err no EP for srv %v", srvID)
		return nil, fmt.Errorf("No EP for srv %v", srvID)
	}
	rpcc, err := rpcncclnt.NewTCPRPCClnt("echosrv", ep)
	if err != nil {
		db.DPrintf(db.COSSIMCLNT_ERR, "Err NewRPCClnt: %v", err)
		return nil, err
	}
	return &CosSimClnt{
		fsl:  fsl,
		rpcc: rpcc,
		epcc: epcc,
	}, nil
}

// Register a service's endpoint
func (clnt *CosSimClnt) CosSim(v []float64, n int64) (uint64, float64, error) {
	db.DPrintf(db.COSSIMCLNT, "CosSim: %v", len(v))
	var res proto.CosSimRep
	req := &proto.CosSimReq{
		InputVec: v,
		N:        n,
	}
	err := clnt.rpcc.RPC("CosSimSrv.CosSim", req, &res)
	if err != nil {
		db.DPrintf(db.COSSIMCLNT_ERR, "Err Register: %v", err)
		return 0, 0.0, err
	}
	db.DPrintf(db.COSSIMCLNT, "CosSim ok: %v -> id:%v val:%v", len(v), res.ID, res.Val)
	return res.ID, res.Val, nil
}
