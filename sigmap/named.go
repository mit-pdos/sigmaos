package sigmap

// if name ends in "/", it is a directory with mount files for that service
const (
	KNAMED        = "knamed"
	NAME          = "name"
	NAMED         = NAME + "/"
	NAMEDREL      = "named"
	MEMFSREL      = "memfs"
	MEMFS         = NAMED + MEMFSREL + "/"
	REALMREL      = "realm"
	REALM         = NAMED + REALMREL + "/"
	REALMDREL     = "realmd"
	REALMD        = NAMED + REALMREL + "/" + REALMDREL
	REALMSREL     = "realms"
	REALMS        = REALMD + "/" + REALMSREL
	BOOTREL       = "boot"
	BOOT          = NAMED + BOOTREL + "/"
	UPROCDREL     = "uprocd"
	S3REL         = "s3"
	S3            = NAMED + S3REL + "/"
	UXREL         = "ux"
	UX            = NAMED + UXREL + "/"
	SCHEDDREL     = "schedd"
	SCHEDD        = NAMED + SCHEDDREL + "/"
	LCSCHEDREL    = "lcsched"
	LCSCHED       = NAMED + LCSCHEDREL + "/"
	SIGMACLNTDREL = "sigmaclntd"
	PROCQREL      = "procq"
	PROCQ         = NAMED + PROCQREL + "/"
	DBREL         = "db"
	DB            = NAMED + DBREL + "/"
	DBD           = DB + "~any/"
	MONGOREL      = "mongo"
	MONGO         = NAMED + MONGOREL + "/"

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

	// Auth
	//	KEYSREL = "keys"
	//	KEYS    = NAME + "/" + KEYSREL
	MASTER_KEY = NAME + "/" + "master-key"

	// special devs/dirs exported by SigmaSrv/SessSrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"
)

// Linux path
const (
	SIGMAHOME   = "/home/sigmaos"
	SIGMASOCKET = "/tmp/sigmaclntd/sigmaclntd.sock"
)
