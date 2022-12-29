package sigmap

// if name ends in "/", it is the root directory for that service (XXX
// it is a union directory?)
const (
	NAMED    = "name/"
	NAMEDREL = "named"
	BOOTREL  = "boot"
	BOOT     = NAMED + BOOTREL + "/"
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
	DBD   = DB + "~local/"

	FWREL = "fw"
	FW    = NAMED + FWREL

	UXBIN = UX + "~local/bin/"

	MEMFS = NAMED + "memfsd/"

	CACHEREL = "cache"
	CACHE    = NAMED + CACHEREL + "/"

	HOTELREL     = "hotel"
	HOTEL        = NAMED + HOTELREL + "/"
	HOTELGEO     = HOTEL + "geo"
	HOTELRATE    = HOTEL + "rate"
	HOTELSEARCH  = HOTEL + "search"
	HOTELREC     = HOTEL + "rec"
	HOTELRESERVE = HOTEL + "reserve"
	HOTELUSER    = HOTEL + "user"
	HOTELPROF    = HOTEL + "prof"

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
	ROOTFS         = "/home/kaashoek/Downloads/rootfs"
	SIGMAROOT      = "/home/sigmaos/"
	PRIVILEGED_BIN = SIGMAROOT + "/bin/"
)

// REALM
const (
	TEST_RID = "test-realm"
)

// SIGMA
const (
	SIGMAMGR = NAMED + "sigmamgr"
)

var HOTELSVC = []string{HOTELGEO, HOTELRATE, HOTELSEARCH, HOTELREC, HOTELRESERVE,
	HOTELUSER, HOTELPROF, DB + "~any/"}
