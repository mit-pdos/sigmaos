package atomic

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/rand"
)

func PutFileAtomic(fsl *fslib.FsLib, fname string, perm np.Tperm, data []byte) error {
	tmpName := rand.String(16)
	tmpPath := path.Join(np.TMP, tmpName)
	_, err := fsl.PutFile(tmpPath, perm, np.OWRITE, data)
	if err != nil {
		log.Fatalf("FATAL Error in MakeFileAtomic %v: %v", fname, err)
		return err
	}
	err = fsl.Rename(tmpPath, fname)
	if err != nil {
		log.Fatalf("FATAL Error in MakeFileAtomic rename %v -> %v: %v", tmpPath, fname, err)
		return err
	}
	return nil
}

func PutFileJsonAtomic(fsl *fslib.FsLib, fname string, perm np.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("FATAL Marshal error %v", err)
	}
	return PutFileAtomic(fsl, fname, perm, data)
}
