package clnt

import (
	"encoding/json"

	"sigmaos/db/proto"
	"sigmaos/fslib"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
)

type DbClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewDbClnt(fsl *fslib.FsLib, pn string) (*DbClnt, error) {
	dc := &DbClnt{}
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, pn)
	if err != nil {
		return nil, err
	}
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
