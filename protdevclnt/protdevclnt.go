package protdevclnt

import (
	"fmt"
	"sigmaos/fslib"
	np "sigmaos/ninep"
)

type ProtDevClnt struct {
	*fslib.FsLib
	sid string
}

func MkProtDevClnt(fsl *fslib.FsLib, fn string) (*ProtDevClnt, error) {
	pdc := &ProtDevClnt{}
	pdc.FsLib = fsl
	b, err := pdc.GetFile(np.DBD + "clone")
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	pdc.sid = string(b)
	return pdc, nil
}

func (pdc *ProtDevClnt) RPC(req []byte) ([]byte, error) {
	_, err := pdc.SetFile(np.DBD+pdc.sid+"/data", req, np.OWRITE, 0)
	if err != nil {
		return nil, fmt.Errorf("Query err %v\n", err)
	}
	// XXX maybe the caller should use Reader
	b, err := pdc.GetFile(np.DBD + pdc.sid + "/data")
	if err != nil {
		return nil, fmt.Errorf("Query response err %v\n", err)
	}
	return b, nil
}
