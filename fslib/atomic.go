package fslib

import (
	"encoding/json"
	"fmt"

	db "sigmaos/debug"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) PutFileAtomic(fname string, perm sp.Tperm, data []byte, lid sp.TleaseId) error {
	tmpName := fname + rand.String(16)
	_, err := fsl.PutFileEphemeral(tmpName, perm, sp.OWRITE|sp.OEXCL, data, lid)
	if err != nil {
		db.DFatalf("MakeFileAtomic %v %v: %v", fname, tmpName, err)
		return err
	}
	err = fsl.Rename(tmpName, fname)
	if err != nil {
		db.DFatalf("MakeFileAtomic rename %v -> %v: err %v", tmpName, fname, err)
		return err
	}
	return nil
}

func (fsl *FsLib) PutFileJsonAtomic(fname string, perm sp.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("PutFileJsonAtomic marshal err %v", err)
	}
	return fsl.PutFileAtomic(fname, perm, data, sp.NoLeaseId)
}
