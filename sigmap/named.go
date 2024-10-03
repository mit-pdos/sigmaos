package sigmap

import (
	"path/filepath"
	"strings"
)

// if name ends in "/", it is a directory with mount files for that service
const (
	KNAMED      = "knamed"
	NAME        = "name"
	ROOT        = "root"
	NAMED       = NAME + "/"
	NAMEDREL    = "named"
	MEMFSREL    = "memfs"
	MEMFS       = NAMED + MEMFSREL + "/"
	REALMREL    = "realm"
	REALM       = NAMED + REALMREL + "/"
	REALMDREL   = "realmd"
	REALMD      = NAMED + REALMREL + "/" + REALMDREL
	REALMSREL   = "realms"
	REALMS      = REALMD + "/" + REALMSREL
	BOOTREL     = "boot"
	BOOT        = NAMED + BOOTREL + "/"
	UPROCDREL   = "uprocd"
	S3REL       = "s3"
	S3          = NAMED + S3REL + "/"
	UXREL       = "ux"
	UX          = NAMED + UXREL + "/"
	CHUNKDREL   = "chunkd"
	CHUNKD      = NAMED + CHUNKDREL + "/"
	SCHEDDREL   = "schedd"
	SCHEDD      = NAMED + SCHEDDREL + "/"
	LCSCHEDREL  = "lcsched"
	LCSCHED     = NAMED + LCSCHEDREL + "/"
	SPPROXYDREL = "spproxyd"
	PROCQREL    = "procq"
	PROCQ       = NAMED + PROCQREL + "/"
	DBREL       = "db"
	DB          = NAMED + DBREL + "/"
	DBD         = DB + "~any/"
	MONGOREL    = "mongo"
	MONGO       = NAMED + MONGOREL + "/"

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

	// names for directly-mounted services
	S3CLNT = "s3clnt"
)

func IsS3Path(pn string) bool {
	return strings.HasPrefix(pn, S3)
}

func ClientPath(pn string) (string, bool) {
	pn0, ok := strings.CutPrefix(pn, filepath.Join(S3, "~local"))
	if ok {
		return filepath.Join(S3CLNT, pn0), true
	}
	return pn, false
}

// Dirs mounted from the root named into tenants' realms
var RootNamedMountedDirs map[string]bool = map[string]bool{
	REALMREL:   true,
	LCSCHEDREL: true,
	PROCQREL:   true,
	SCHEDDREL:  true,
	BOOTREL:    true,
	DBREL:      true,
	MONGOREL:   true,
}

// Linux path
const (
	SIGMAHOME             = "/home/sigmaos"
	SIGMASOCKET           = "/tmp/spproxyd/spproxyd.sock"
	SIGMA_NETPROXY_SOCKET = "/tmp/spproxyd/spproxyd-netproxy.sock"
)

// spproxyd kernel
const (
	SPPROXYDKERNEL = "kernel-" + SPPROXYDREL + "-"
	PROCQKERNEL    = "kernel-" + PROCQREL + "-"
)

func SPProxydKernel(kid string) string {
	return SPPROXYDKERNEL + kid
}

func ProcqKernel(kid string) string {
	return PROCQKERNEL + kid
}

func ProxyPathname(srv, kid string) string {
	return filepath.Join(srv, kid)
}
