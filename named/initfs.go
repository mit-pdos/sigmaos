package named

import (
	"ulambda/fslib"
)

const (
	NAMED         = "name"
	LOCKS         = "name/locks"
	BOOT          = "name/boot"
	TMP           = "name/tmp"
	PROCD         = "name/procd"
	S3            = "name/s3"
	UX            = "name/ux"
	FS            = "name/fs"
	DB            = "name/db"
	PROC_COND     = "name/proc-cond"
	PROC_RET_STAT = "name/proc-ret-stat"
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
	if err := fsl.Mkdir(FS, 0777); err != nil {
		return err
	}
	if err := fsl.Mkdir(PROC_COND, 0777); err != nil {
		return err
	}
	if err := fsl.Mkdir(PROC_RET_STAT, 0777); err != nil {
		return err
	}
	return nil
}
