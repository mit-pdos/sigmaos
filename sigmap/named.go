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
	CHUNKDREL     = "chunkd"
	CHUNKD        = NAMED + CHUNKDREL + "/"
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

	// Uprocd
	PUBLIC_HTTP_PORT  = "public-http-port"
	PUBLIC_NAMED_PORT = "public-named-port"

	// Auth
	KEYDREL    = "keyd"
	KEYD       = NAME + "/" + KEYDREL
	RW_REL     = "rw"
	RONLY_REL  = "ronly"
	KEYS_RW    = KEYD + "/" + RW_REL
	KEYS_RONLY = KEYD + "/" + RONLY_REL

	// special devs/dirs exported by SigmaSrv/SessSrv
	STATSD   = ".statsd"
	FENCEDIR = ".fences"
	SNAPDEV  = "snapdev"

	// stats exported by named
	PSTATSD = ".pstatsd"
)

// Linux path
const (
	SIGMAHOME             = "/home/sigmaos"
	SIGMASOCKET           = "/tmp/sigmaclntd/sigmaclntd.sock"
	SIGMA_NETPROXY_SOCKET = "/tmp/sigmaclntd/sigmaclntd-netproxy.sock"
)

// sigmaclntd kernel
const (
	SIGMACLNTDKERNEL = "kernel-" + SIGMACLNTDREL + "-"
)

func SigmaClntdKernel(kid string) string {
	return SIGMACLNTDKERNEL + kid
}
