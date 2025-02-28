package fslib

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/rand"
)

func (fsl *FsLib) PutFileAtomic(pn sp.Tsigmapath, perm sp.Tperm, data []byte) error {
	tmpName := pn + rand.Name()
	if _, err := fsl.PutFile(tmpName, perm, sp.OWRITE|sp.OEXCL, data); err != nil {
		debug.PrintStack()
		db.DPrintf(db.ERROR, "PutFileAtomic %v %v err %v", pn, tmpName, err)
		return err
	}
	if err := fsl.Rename(tmpName, pn); err != nil {
		db.DPrintf(db.ERROR, "PutFileAtomic rename %v -> %v: err %v", tmpName, pn, err)
		return err
	}
	return nil
}

func (fsl *FsLib) PutFileJsonAtomic(pn sp.Tsigmapath, perm sp.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("PutFileJsonAtomic marshal err %v", err)
	}
	return fsl.PutFileAtomic(pn, perm, data)
}
