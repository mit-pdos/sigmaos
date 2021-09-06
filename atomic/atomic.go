package atomic

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"runtime/debug"

	"github.com/thanhpk/randstr"

	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
)

// TODO: Remove with OCREAT
const (
	JSON_PADDING = 500
)

func MakeFileAtomic(fsl *fslib.FsLib, fname string, perm np.Tperm, data []byte) error {
	tmpName := randstr.Hex(16)
	tmpPath := path.Join(named.TMP, tmpName)
	err := fsl.MakeFile(tmpPath, perm, np.OWRITE, data)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("Error in MakeFileAtomic %v: %v", fname, err)
		return err
	}
	err = fsl.Rename(tmpPath, fname)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("Error in MakeFileAtomic rename %v -> %v: %v", tmpPath, fname, err)
		return err
	}
	return err
}

func MakeFileJsonAtomic(fsl *fslib.FsLib, fname string, perm np.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	for i := 0; i < JSON_PADDING; i++ {
		data = append(data, ' ')
	}
	return MakeFileAtomic(fsl, fname, perm, data)
}
