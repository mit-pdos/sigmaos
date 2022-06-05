package ninep

// if name ends in "/", it is the root directory for that service
const (
	NAMED    = "name/"
	BOOTREL  = "boot"
	BOOT     = NAMED + BOOTREL
	TMPREL   = "tmp"
	TMP      = NAMED + TMPREL
	PROCDREL = "procd"
	PROCD    = NAMED + PROCDREL + "/"
	PROCD_WS = PROCD + "ws" + "/"
	S3REL    = "s3"
	S3       = NAMED + S3REL + "/"
	UXREL    = "ux"
	UX       = NAMED + UXREL + "/"
	DBREL    = "db"
	DB       = NAMED + DBREL + "/"

	BIN = UX + "~ip/bin/"

	SIGMAMGR = "name/sigmamgr"

	MEMFS = "name/memfsd/"

	KPIDSREL = "kpids"
	KPIDS    = NAMED + KPIDSREL

	PROC_CTL_FILE = "ctl"
	PROCD_RUNNING = "running"
	PROCD_RUNQ_LC = "runq-lc"
	PROCD_RUNQ_BE = "runq-be"

	// special devs/dirs exported by fssrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"

	UXEXPORT = "/tmp/ulambda"
)

// REALM
const (
	TEST_RID = "test-realm"
)
