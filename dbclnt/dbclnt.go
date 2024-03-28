package dbclnt

import (
	"encoding/json"

	"sigmaos/dbsrv/proto"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/sigmarpcchan"
)

type DbClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewDbClnt(fsl *fslib.FsLib, fn string) (*DbClnt, error) {
	dc := &DbClnt{}
	ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{fsl}, fn)
	if err != nil {
		return nil, err
	}
	rpcc := rpcclnt.NewRPCClnt(ch)
	dc.rpcc = rpcc
	return dc, nil
}

func (dc *DbClnt) Query(q string, res any) error {
	req := &proto.DBRequest{Cmd: q}
	rep := proto.DBResult{}
	err := dc.rpcc.RPC("Server.Query", req, &rep)
	if err != nil {
		return err
	}
	err = json.Unmarshal(rep.Res, res)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DbClnt) Exec(q string) error {
	req := &proto.DBRequest{Cmd: q}
	rep := proto.DBResult{}
	err := dc.rpcc.RPC("Server.Exec", req, &rep)
	if err != nil {
		return err
	}
	return nil
}
