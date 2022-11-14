package dbclnt

import (
	"encoding/json"
	"fmt"

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

func (dc *DbClnt) Query(q string, v any) error {
	b, err := dc.pdc.RPC([]byte(q))
	if err != nil {
		return fmt.Errorf("Query err %v\n", err)
	}
	err = json.Unmarshal(b, v)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DbClnt) Exec(q string) error {
	_, err := dc.pdc.RPC([]byte(q))
	if err != nil {
		return fmt.Errorf("Query err %v\n", err)
	}
	return nil
}
