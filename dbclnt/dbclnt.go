package dbclnt

import (
	"encoding/json"

	"sigmaos/dbd/proto"
	"sigmaos/fslib"
	"sigmaos/protdevclnt"
)

type DbClnt struct {
	pdc *protdevclnt.ProtDevClnt
}

func MkDbClnt(fsl *fslib.FsLib, fn string) (*DbClnt, error) {
	dc := &DbClnt{}
	pdc, err := protdevclnt.MkProtDevClnt(fsl, fn)
	if err != nil {
		return nil, err
	}
	dc.pdc = pdc
	return dc, nil
}

func (dc *DbClnt) Query(q string, res any) error {
	req := &proto.DBRequest{Cmd: q}
	rep := proto.DBResult{}
	err := dc.pdc.RPC("Server.Query", req, &rep)
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
	err := dc.pdc.RPC("Server.Exec", req, &rep)
	if err != nil {
		return err
	}
	return nil
}
