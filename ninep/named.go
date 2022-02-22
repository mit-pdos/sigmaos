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
	S3REL         = "s3"
	S3            = "name/" + S3REL + "/"
	UXREL         = "ux"
	UX            = "name/" + UXREL + "/"
	DBREL         = "db"
	DB            = "name/" + DBREL + "/"
	REALM_MGR     = "name/realmmgr"
	MEMFS         = "name/memfsd/"
	KPIDSREL      = "kpids"
	KPIDS         = "name/" + KPIDSREL
	PROC_CTL_FILE = "ctl"
	PROCD_RUNNING = "running"
	PROCD_RUNQ_LC = "runq-lc"
	PROCD_RUNQ_BE = "runq-be"
)
