package named

import (
	"ulambda/fslib"
)

// if name ends in "/", it is a directory for that service
const (
	NAMED         = "name"
	LOCKS         = "name/locks"
	BOOT          = "name/boot"
	TMP           = "name/tmp"
	PROCDDIR      = "procd"
	PROCD         = "name/" + PROCDDIR + "/"
	S3            = "name/s3/"
	UX            = "name/ux/"
	DB            = "name/db/"
	REALM_MGR     = "name/realmmgr"
	MEMFS         = "name/memfsd/"
	PIDS          = "name/pids"
	PROC_CTL_FILE = "ctl"
	PROCD_RUNNING = "running"
	PROCD_RUNQ_LC = "runq-lc"
	PROCD_RUNQ_BE = "runq-be"
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
	if err := fsl.Mkdir(PIDS, 0777); err != nil {
		return err
	}
	if err := fsl.Mkdir(PROCD, 0777); err != nil {
		return err
	}
	return nil
}
