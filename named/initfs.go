package named

import (
	"ulambda/fslib"
)

const (
	NAMED = "name"
	LOCKS = "name/locks"
	TMP   = "name/tmp"
	PROCD = "name/procd"
	S3    = "name/s3"
	UX    = "name/ux"
	BOOT  = "name/boot"
)

func MakeInitFs(fsl *fslib.FsLib) error {
	if err := fsl.Mkdir(LOCKS, 0777); err != nil {
		return err
	}
	if err := fsl.Mkdir(TMP, 0777); err != nil {
		return err
	}
	if err := fsl.Mkdir(BOOT, 0777); err != nil {
		return err
	}
	return nil
}
