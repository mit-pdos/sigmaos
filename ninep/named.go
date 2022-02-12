package ninep

// if name ends in "/", it is a directory for that service
const (
	NAMED         = "name"
	BOOTREL       = "boot"
	BOOT          = "name/" + BOOTREL
	TMPREL        = "tmp"
	TMP           = "name/" + TMPREL
	PROCDREL      = "procd"
	PROCD         = "name/" + PROCDREL + "/"
	S3            = "name/s3/"
	UX            = "name/ux/"
	DB            = "name/db/"
	REALM_MGR     = "name/realmmgr"
	MEMFS         = "name/memfsd/"
	PIDSREL       = "pids"
	PIDS          = "name/" + PIDSREL
	KPIDSREL      = "kpids"
	KPIDS         = "name/" + KPIDSREL
	PROC_CTL_FILE = "ctl"
	PROCD_RUNNING = "running"
	PROCD_RUNQ_LC = "runq-lc"
	PROCD_RUNQ_BE = "runq-be"
)
