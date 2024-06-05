package fslib

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) PutFileAtomic(fname string, perm sp.Tperm, data []byte) error {
	tmpName := fname + rand.String(16)
	if _, err := fsl.PutFile(tmpName, perm, sp.OWRITE|sp.OEXCL, data); err != nil {
		debug.PrintStack()
		db.DPrintf(db.ERROR, "PutFileAtomic %v %v err %v", fname, tmpName, err)
		return err
	}
	if err := fsl.Rename(tmpName, fname); err != nil {
		db.DPrintf(db.ERROR, "PutFileAtomic rename %v -> %v: err %v", tmpName, fname, err)
		return err
	}
	return nil
}

func (fsl *FsLib) PutFileJsonAtomic(fname string, perm sp.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("PutFileJsonAtomic marshal err %v", err)
	}
	return fsl.PutFileAtomic(fname, perm, data)
}
