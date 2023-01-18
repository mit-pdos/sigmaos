package sigmap

// if name ends in "/", it is the root directory for that service (XXX
// it is a union directory?)
const (
	NAMED       = "name/"
	NAMEDREL    = "named"
	BOOTREL     = "boot"
	BOOT        = NAMED + BOOTREL + "/"
	TMPREL      = "tmp"
	TMP         = NAMED + TMPREL
	PROCDREL    = "procd"
	PROCD       = NAMED + PROCDREL + "/"
	S3REL       = "s3"
	S3          = NAMED + S3REL + "/"
	UXREL       = "ux"
	UX          = NAMED + UXREL + "/"
	SCHEDDREL   = "schedd"
	SCHEDD      = NAMED + SCHEDDREL + "/"
	SIGMAMGRREL = "sigmamgr"
	SIGMAMGR    = NAMED + SIGMAMGRREL + "/"
	DBREL       = "db"
	DB          = NAMED + DBREL + "/"
	DBD         = DB + "~local/"

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

	// Schedd
	QUEUE          = "queue"
	WS             = "name/" + WS_REL
	WS_REL         = "ws/"
	WS_RUNQ_LC_REL = WS_REL + "runq-lc/"
	WS_RUNQ_BE_REL = WS_REL + "runq-be/"
	WS_RUNQ_LC     = WS + "runq-lc/"
	WS_RUNQ_BE     = WS + "runq-be/"

	// Procd spawn file
	PROCD_SPAWN_FILE = "spawn"

	PROCD_RUNNING = "running"

	// special devs/dirs exported by fssrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"

	// Resource
	RESOURCE_CTL = "resourcectl"
)

// Linux path
const (
	SIGMAHOME = "/home/sigmaos"
)

// REALM
const (
	TEST_RID = "testrealm"
)

var HOTELSVC = []string{HOTELGEO, HOTELRATE, HOTELSEARCH, HOTELREC, HOTELRESERVE,
	HOTELUSER, HOTELPROF, DB + "~any/"}
