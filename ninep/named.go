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

	DBREL = "db"
	DB    = NAMED + DBREL + "/"
	DBD   = DB + "~ip/"

	UXBIN = UX + "~ip/bin/"

	SIGMAMGR = NAMED + "sigmamgr"

	MEMFS = NAMED + "memfsd/"

	HOTELREL  = "hotel"
	HOTEL     = NAMED + HOTELREL + "/"
	HOTELGEO  = HOTEL + "geo"
	HOTELRATE = HOTEL + "rate"

	KPIDSREL = "kpids"
	KPIDS    = NAMED + KPIDSREL

	// Procd spawn file
	PROCD_SPAWN_FILE = "spawn"

	PROCD_RUNNING = "running"
	PROCD_RUNQ_LC = "runq-lc"
	PROCD_RUNQ_BE = "runq-be"

	// special devs/dirs exported by fssrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"

	// Resource
	RESOURCE_CTL = "resourcectl"
)

// Linux paths
const (
	UXROOT         = "/tmp/sigmaos/"
	PRIVILEGED_BIN = UXROOT + "bin/"
)

// REALM
const (
	TEST_RID = "test-realm"
)

// SIGMA
const (
	SIGMACTL = SIGMAMGR + "/" + RESOURCE_CTL
)
