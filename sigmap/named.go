package sigmap

// if name ends in "/", it is the union directory for that service
const (
	KNAMED    = "knamed"
	NAME      = "name"
	NAMED     = NAME + "/"
	NAMEDREL  = "named"
	REALMDREL = "realmd"
	REALMD    = NAMED + REALMDREL
	REALMSREL = "realms"
	REALMS    = NAMED + REALMDREL + "/" + REALMSREL
	BOOTREL   = "boot"
	BOOT      = NAMED + BOOTREL + "/"
	UPROCDREL = "uprocd"
	S3REL     = "s3"
	S3        = NAMED + S3REL + "/"
	UXREL     = "ux"
	UX        = NAMED + UXREL + "/"
	SCHEDDREL = "schedd"
	SCHEDD    = NAMED + SCHEDDREL + "/"
	DBREL     = "db"
	DB        = NAMED + DBREL + "/"
	DBD       = DB + "~any/"
	MONGOREL  = "mongo"
	MONGO     = NAMED + MONGOREL + "/"
	MONGOD    = MONGO + "~any/"

	UXBIN = UX + "~local/bin/"

	IMGREL = "img"
	IMG    = NAMED + IMGREL + "/"

	MEMCACHED = "name/memcached"
	MEMBLOCK  = "name/memblock"

	SOCIAL_NETWORK          = NAMED + "socialnetwork/"
	SOCIAL_NETWORK_USER     = SOCIAL_NETWORK + "user"
	SOCIAL_NETWORK_GRAPH    = SOCIAL_NETWORK + "graph"
	SOCIAL_NETWORK_POST     = SOCIAL_NETWORK + "post"
	SOCIAL_NETWORK_TIMELINE = SOCIAL_NETWORK + "timeline"
	SOCIAL_NETWORK_HOME     = SOCIAL_NETWORK + "home"
	SOCIAL_NETWORK_URL      = SOCIAL_NETWORK + "url"
	SOCIAL_NETWORK_TEXT     = SOCIAL_NETWORK + "text"
	SOCIAL_NETWORK_COMPOSE  = SOCIAL_NETWORK + "compose"
	SOCIAL_NETWORK_MEDIA    = SOCIAL_NETWORK + "media"

	K8S_SCRAPER = NAMED + "k8sscraper/"

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

	// special devs/dirs exported by SigmaSrv/SessSrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"
)

// Linux path
const (
	SIGMAHOME = "/home/sigmaos"
)
