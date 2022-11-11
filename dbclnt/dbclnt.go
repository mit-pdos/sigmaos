package dbclnt

import (
	"fmt"

	"sigmaos/fslib"
	np "sigmaos/ninep"
)

type DbClnt struct {
	*fslib.FsLib
}

func MkDbClnt(fsl *fslib.FsLib) *DbClnt {
	dc := &DbClnt{}
	dc.FsLib = fsl
	return dc
}

func (dc *DbClnt) Query(q string) ([]byte, error) {
	b, err := dc.GetFile(np.DBD + "clone")
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	sid := string(b)
	_, err = dc.SetFile(np.DBD+sid+"/query", []byte(q), np.OWRITE, 0)
	if err != nil {
		return nil, fmt.Errorf("Query err %v\n", err)
	}
	// XXX maybe the caller should use Reader
	b, err = dc.GetFile(np.DBD + sid + "/data")
	if err != nil {
		return nil, fmt.Errorf("Query response err %v\n", err)
	}
	return b, nil
}
