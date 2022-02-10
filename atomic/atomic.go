package atomic

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"runtime/debug"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/rand"
)

func MakeFileAtomic(fsl *fslib.FsLib, fname string, perm np.Tperm, data []byte) error {
	tmpName := rand.String(16)
	tmpPath := path.Join(np.TMP, tmpName)
	err := fsl.MakeFile(tmpPath, perm, np.OWRITE, data)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL Error in MakeFileAtomic %v: %v", fname, err)
		return err
	}
	err = fsl.Rename(tmpPath, fname)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL Error in MakeFileAtomic rename %v -> %v: %v", tmpPath, fname, err)
		return err
	}
	return err
}

func MakeFileJsonAtomic(fsl *fslib.FsLib, fname string, perm np.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("FATAL Marshal error %v", err)
	}
	return MakeFileAtomic(fsl, fname, perm, data)
}
