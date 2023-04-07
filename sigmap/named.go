package sigmap

// if name ends in "/", it is the root directory for that service (XXX
// it is a union directory?)
const (
	NAMED       = "name/"
	NAMEDREL    = "named"
	REALMDREL   = "realmd"
	REALMD      = NAMED + REALMDREL
	REALMSREL   = "realms"
	REALMS      = NAMED + REALMDREL + "/" + REALMSREL
	BOOTREL     = "boot"
	BOOT        = NAMED + BOOTREL + "/"
	TMPREL      = "tmp"
	TMP         = NAMED + TMPREL
	UPROCDREL   = "uprocd"
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
	DBD         = DB + "~any/"

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

	SOCIAL_NETWORK          = NAMED + "socialnetwork/"
	SOCIAL_NETWORK_MOL      = SOCIAL_NETWORK + "mol"
	SOCIAL_NETWORK_USER     = SOCIAL_NETWORK + "user"

	KPIDSREL = "kpids"
	KPIDS    = NAMED + KPIDSREL

	// Schedd
	QUEUE          = "queue"
	RUNNING        = "running"
	PIDS           = "pids"
	WS             = "name/" + WS_REL + "/"
	WS_REL         = "ws"
	WS_RUNQ_LC_REL = "runq-lc"
	WS_RUNQ_BE_REL = "runq-be"
	WS_RUNQ_LC     = WS + WS_RUNQ_LC_REL + "/"
	WS_RUNQ_BE     = WS + WS_RUNQ_BE_REL + "/"

	// special devs/dirs exported by fssrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"
)

// Linux path
const (
	SIGMAHOME = "/home/sigmaos"
)

var HOTELSVC = []string{HOTELGEO, HOTELRATE, HOTELSEARCH, HOTELREC, HOTELRESERVE,
	HOTELUSER, HOTELPROF, DB + "~any/"}
