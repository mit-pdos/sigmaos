package dbclnt

import (
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

func (dc *DbClnt) Query(q string) ([]byte, error) {
	b, err := dc.pdc.RPC([]byte(q))
	if err != nil {
		return nil, fmt.Errorf("Query response err %v\n", err)
	}
	return b, nil
}
