package dbclnt

import (
	"encoding/json"

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
	r := new([]uint8)
	err := dc.pdc.RPC("Server.Query", q, r)
	if err != nil {
		return err
	}
	err = json.Unmarshal(*r, res)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DbClnt) Exec(q string) error {
	r := new([]uint8)
	err := dc.pdc.RPC("Server.Exec", q, r)
	if err != nil {
		return err
	}
	return nil
}
