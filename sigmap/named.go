package sigmap

// if name ends in "/", it is the union directory for that service
const (
	KNAMED     = "knamed"
	NAME       = "name"
	NAMED      = NAME + "/"
	NAMEDREL   = "named"
	REALMDREL  = "realmd"
	REALMD     = NAMED + REALMDREL
	REALMSREL  = "realms"
	REALMS     = NAMED + REALMDREL + "/" + REALMSREL
	BOOTREL    = "boot"
	BOOT       = NAMED + BOOTREL + "/"
	UPROCDREL  = "uprocd"
	S3REL      = "s3"
	S3         = NAMED + S3REL + "/"
	UXREL      = "ux"
	UX         = NAMED + UXREL + "/"
	SCHEDDREL  = "schedd"
	SCHEDD     = NAMED + SCHEDDREL + "/"
	LCSCHEDREL = "lcsched"
	LCSCHED    = NAMED + LCSCHEDREL + "/"
	PROCQREL   = "procq"
	PROCQ      = NAMED + PROCQREL + "/"
	DBREL      = "db"
	DB         = NAMED + DBREL + "/"
	DBD        = DB + "~any/"
	MONGOREL   = "mongo"
	MONGO      = NAMED + MONGOREL + "/"

	UXBIN = UX + "~local/bin/"

	IMGREL = "img"
	IMG    = NAMED + IMGREL + "/"

	MEMCACHED = "name/memcached"
	MEMBLOCK  = "name/memblock"

	K8S_SCRAPER = NAMED + "k8sscraper/"

	KPIDSREL = "kpids"
	KPIDS    = NAMED + KPIDSREL

	// Schedd
	QUEUE   = "queue"
	RUNNING = "running"
	PIDS    = "pids"

	// special devs/dirs exported by SigmaSrv/SessSrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"
)

// Linux path
const (
	SIGMAHOME = "/home/sigmaos"
)
