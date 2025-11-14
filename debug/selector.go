package debug

type Tselector string

// ALWAYS
const (
	ALWAYS Tselector = "ALWAYS"
	ERROR  Tselector = "ERROR"
	NEVER  Tselector = "NEVER"
)

// ERR
const (
	ERR Tselector = "_ERR"
)

// Benchmarks
const (
	LOADGEN        Tselector = "LOADGEN"
	BENCH          Tselector = "BENCH"
	THROUGHPUT     Tselector = "THROUGHPUT"
	CPU_UTIL       Tselector = "CPU_UTIL"
	AUTOSCALER     Tselector = "AUTOSCALER"
	AUTOSCALER_ERR Tselector = AUTOSCALER + ERR
)

// Latency break-down.
const (
	SPAWN_LAT         Tselector = "SPAWN_LAT"
	SPAWN_LAT_VERBOSE Tselector = "SPAWN_LAT_VERBOSE"
	NET_LAT           Tselector = "NET_LAT"
	DIALPROXY_LAT     Tselector = "DIALPROXY_LAT"
	REALM_GROW_LAT    Tselector = "REALM_GROW_LAT"
	CACHE_LAT         Tselector = "CACHE_LAT"
	WALK_LAT          Tselector = "WALK_LAT"
	CLUNK_LAT         Tselector = "CLUNK_LAT"
	FSETCD_LAT        Tselector = "FSETCD_LAT"
	ATTACH_LAT        Tselector = "ATTACH_LAT"
	RPC_LAT           Tselector = "RPC_LAT"
	PROXY_RPC_LAT     Tselector = "PROXY_RPC_LAT"
)

// Tests
const (
	TEST     Tselector = "TEST"
	TEST1    Tselector = "TEST1"
	STAT     Tselector = "STAT"
	TEST_LAT Tselector = "TEST_LAT"
	DELAY    Tselector = "DELAY"
	CRASH    Tselector = "CRASH"
	PERF     Tselector = "PERF"
)

// Cache
const (
	CACHESRV      Tselector = "CACHESRV"
	CACHESRV_ERR  Tselector = CACHESRV + ERR
	CACHECLERK    Tselector = "CACHECLERK"
	CACHEDSVCCLNT Tselector = "CACHEDSVCCLNT"
)

// CosSim
const (
	COSSIMSRV      Tselector = "COSSIMSRV"
	COSSIMSRV_ERR  Tselector = COSSIMSRV + ERR
	COSSIMCLNT     Tselector = "COSSIMCLNT"
	COSSIMCLNT_ERR Tselector = COSSIMCLNT + ERR
)

// EPCache
const (
	EPCACHE         Tselector = "EPCACHE"
	EPCACHE_ERR     Tselector = EPCACHE + ERR
	EPCACHECLNT     Tselector = "EPCACHECLNT"
	EPCACHECLNT_ERR Tselector = EPCACHECLNT + ERR
)

// Hotel
const (
	HOTEL_CLNT      Tselector = "HOTEL_CLNT"
	HOTEL_GEO       Tselector = "HOTEL_GEO"
	HOTEL_GEO_ERR   Tselector = "HOTEL_GEO" + ERR
	HOTEL_PROF      Tselector = "HOTEL_PROF"
	HOTEL_RATE      Tselector = "HOTEL_RATE"
	HOTEL_RESERVE   Tselector = "HOTEL_RESERVE"
	HOTEL_SEARCH    Tselector = "HOTEL_SEARCH"
	HOTEL_MATCH     Tselector = "HOTEL_MATCH"
	HOTEL_MATCH_ERR Tselector = "HOTEL_MATCH" + ERR
	HOTEL_WWW       Tselector = "HOTEL_WWW"
	HOTEL_WWW_ERR   Tselector = "HOTEL_WWW" + ERR
	HOTEL_WWW_STATS Tselector = "HOTEL_WWW_STATS"
)

// Test apps
const (
	SLEEPER        Tselector = "SLEEPER"
	SPINNER        Tselector = "SPINNER"
	FSREADER       Tselector = "FSREADER"
	SLEEPER_TIMING Tselector = "SLEEPER_TIMING"
	MATMUL         Tselector = "MATMUL"
)

// Img
const (
	IMGD     Tselector = "IMGD"
	IMGD_ERR           = "IMGD" + ERR
)

// MR
const (
	MR       Tselector = "MR"
	MR_COORD Tselector = "MR_COORD"
	MR_TPT             = "MR_TPT"
)

// Socialnet
const (
	SOCIAL_NETWORK          Tselector = "SOCIAL_NETWORK"
	SOCIAL_NETWORK_USER               = SOCIAL_NETWORK + "_USER"
	SOCIAL_NETWORK_GRAPH              = SOCIAL_NETWORK + "_GRAPH"
	SOCIAL_NETWORK_POST               = SOCIAL_NETWORK + "_POST"
	SOCIAL_NETWORK_TIMELINE           = SOCIAL_NETWORK + "_TIMELINE"
	SOCIAL_NETWORK_HOME               = SOCIAL_NETWORK + "_HOME"
	SOCIAL_NETWORK_COMPOSE            = SOCIAL_NETWORK + "_COMPOSE"
	SOCIAL_NETWORK_TEXT               = SOCIAL_NETWORK + "_TEXT"
	SOCIAL_NETWORK_URL                = SOCIAL_NETWORK + "_URL"
	SOCIAL_NETWORK_MEDIA              = SOCIAL_NETWORK + "_MEDIA"
	SOCIAL_NETWORK_FRONTEND           = SOCIAL_NETWORK + "_FRONTEND"
	SOCIAL_NETWORK_CLNT               = SOCIAL_NETWORK + "_CLNT"
)

// Watch
const (
	WATCH_TEST = "WATCH_TEST"
	WATCH_PERF = "WATCH_PERF"
)

// Kernel
const (
	KERNEL         Tselector = "KERNEL"
	KERNELCLNT     Tselector = "KERNELCLNT"
	KERNELCLNT_ERR Tselector = "KERNELCLNT_ERR"
)

// Boot
const (
	BOOTCLNT  Tselector = "BOOTCLNT"
	BOOT      Tselector = "BOOT"
	CONTAINER Tselector = "CONTAINER"
)

// Named
const (
	NAMED     Tselector = "NAMED"
	NAMED_LDR Tselector = "NAMED_LDR"
	FSETCD    Tselector = "FSETCD"
)

// MSched
const (
	PROCDMGR       Tselector = "PROCDMGR"
	PROCDMGR_ERR   Tselector = "PROCDMGR" + ERR
	PROCD          Tselector = "PROCD"
	PROCD_ERR      Tselector = "PROCD" + ERR
	MSCHED         Tselector = "MSCHED"
	MSCHED_ERR     Tselector = "MSCHED" + ERR
	MSCHEDCLNT     Tselector = "MSCHEDCLNT"
	MSCHEDCLNT_ERR Tselector = "MSCHEDCLNT" + ERR
	MSCHED_PERF    Tselector = "MSCHED_PERF"
	CGROUP         Tselector = "CGROUP"
	CGROUP_ERR     Tselector = "CGROUP" + ERR
)

// LCSched
const (
	LCSCHEDCLNT     Tselector = "LCSCHEDCLNT"
	LCSCHEDCLNT_ERR           = "LCSCHEDCLNT" + ERR
	LCSCHED                   = "LCSCHED"
	LCSCHED_ERR               = "LCSCHED" + ERR
)

// BESched
const (
	BESCHED         Tselector = "BESCHED"
	BESCHED_ERR               = "BESCHED" + ERR
	BESCHED_PERF              = "BESCHED_PERF"
	BESCHEDCLNT               = "BESCHEDCLNT"
	BESCHEDCLNT_ERR           = "BESCHEDCLNT" + ERR
)

// Bins
const (
	BINSRV        Tselector = "BINSRV"
	CHUNKSRV                = "CHUNKSRV"
	CHUNKSRV_ERR            = "CHUNKSRV" + ERR
	CHUNKCLNT               = "CHUNKCLNT"
	CHUNKCLNT_ERR           = "CHUNKCLNT" + ERR
)

// Proxies
const (
	S3             Tselector = "S3"
	S3_ERR                   = S3 + ERR
	UX                       = "UX"
	DB                       = "DB"
	MONGO                    = "MONGO"
	MONGO_ERR                = "MONGO" + ERR
	NPPROXY                  = "NPPROXY"
	SPPROXYSRV               = "SPPROXYSRV"
	SPPROXYSRV_ERR           = "SPPROXYSRV" + ERR
)

// Realm
const (
	SIGMAMGR     Tselector = "SIGMAMGR"
	SIGMAMGR_ERR           = SIGMAMGR + ERR
	FAIRNESS               = "FAIRNESS"
	REALMD                 = "REALMD"
	REALMD_ERR             = "REALMD" + ERR
	REALMMGR               = "REALMMGR"
	REALMMGR_ERR           = REALMMGR + ERR
	REALMCLNT              = "REALMCLNT"
	REALM_LOCK             = "REALM_LOCK"
)

// Client Libraries
const (
	WRITER_ERR   Tselector = "WRITER" + ERR
	SIGMACLNT              = "SIGMACLNT"
	READER_ERR             = "READER" + ERR
	AWRITER                = "AWRITER"
	PREADER                = "PREADER"
	FSLIB                  = "FSLIB"
	FSLIB_ERR              = "FSLIB" + ERR
	FIDCLNT                = "FIDCLNT"
	FIDCLNT_ERR            = FIDCLNT + ERR
	FSCLNT                 = "FSCLNT"
	SEMCLNT                = "SEMCLNT"
	SEMCLNT_ERR            = SEMCLNT + ERR
	PROCCLNT               = "PROCCLNT"
	PROCCLNT_ERR           = "PROCCLNT" + ERR
	SPPROXYCLNT            = "SPPROXYCLNT"
)

// Fault-tolerance
const (
	FENCECLNT     Tselector = "FENCECLNT"
	FENCECLNT_ERR           = FENCECLNT + ERR
	GROUPMGR                = "GROUPMGR"
	GROUPMGR_ERR            = GROUPMGR + ERR
	LEADER                  = "LEADER"
	LEADER_ERR              = LEADER + ERR
	LEASECLNT               = "LEASECLNT"
	ELECTCLNT               = "ELECTCLNT"
	EPOCHCLNT               = "EPOCHCLNT"
	EPOCHCLNT_ERR           = EPOCHCLNT + ERR
	FTTASKSRV               = "FTTASKSRV"
	FTTASKCLNT              = "FTTASKCLNT"
	FTTASKMGR               = "FTTASKMGR"
)

// RPC Client Libraries
const (
	DEMUXCLNT       Tselector = "DEMUXCLNT"
	DEMUXCLNT_ERR             = "DEMUXCLNT" + ERR
	SESSDEVCLNT               = "SESSDEVCLNT"
	SESSDEVCLNT_ERR           = "SESSDEVCLNT" + ERR
	RPCCLNT                   = "RPCCLNT"
	RPCCHAN                   = "RPCCHAN"
)

// External service libraries
const (
	S3CLNT   Tselector = "S3CLNT"
	K8S_UTIL           = "K8S_UTIL"
)

// Server Libraries
const (
	MEMFS            Tselector = "MEMFS"
	PIPE                       = "PIPE"
	OVERLAYDIR                 = "OVERLAYDIR"
	CLONEDEV                   = "CLONEDEV"
	SESSDEV                    = "SESSDEV"
	SIGMASRV                   = "SIGMASRV"
	DIALPROXY                  = "DIALPROXY"
	DIALPROXY_ERR              = "DIALPROXY" + ERR
	DIALPROXYSRV               = "DIALPROXYSRV"
	DIALPROXYSRV_ERR           = "DIALPROXYSRV" + ERR
	SHMEM                      = "SHMEM"
	SHMEM_ERR                  = "SHMEM" + ERR
)

// Networking
const (
	NETCLNT            Tselector = "NETCLNT"
	NETCLNT_ERR                  = NETCLNT + ERR
	DIALPROXYCLNT                = "DIALPROXYCLNT"
	DIALPROXYCLNT_ERR            = "DIALPROXYCLNT" + ERR
	DIALPROXYTRANS               = "DIALPROXYTRANS"
	DIALPROXYTRANS_ERR           = "DIALPROXYTRANS" + ERR
)

// Path resolution
const (
	PATHCLNT     Tselector = "PATHCLNT"
	PATHCLNT_ERR           = PATHCLNT + ERR
	WALK                   = "WALK"
	WALK_ERR               = "WALK" + ERR
	MOUNT                  = "MOUNT"
	MOUNT_ERR              = MOUNT + ERR
)

// Sessions & protocol client-side infrastructure
const (
	PROTCLNT     Tselector = "PROTCLNT"
	PROTCLNT_ERR           = "PROTCLNT" + ERR
	SESS_CLNT_Q            = "SESS_CLNT_Q"
	SESSCLNT               = "SESSCLNT"
	SESSCLNT_ERR           = SESSCLNT + ERR
)

// Server-side Infrastructure
const (
	NETSRV        Tselector = "NETSRV"
	DEMUXSRV                = "DEMUXSRV"
	DEMUXSRV_ERR            = "DEMUXSRV" + ERR
	REPLRAFT                = "REPLRAFT"
	RAFT_TIMING             = "RAFT_TIMING"
	REPLY_TABLE             = "REPLY_TABLE"
	INTERVALS               = "INTERVALS"
	SESSSRV                 = "SESSSRV"
	WATCH                   = "WATCH"
	WATCH_ERR               = WATCH + ERR
	LOCKMAP                 = "LOCKMAP"
	SNAP                    = "SNAP"
	NAMEI                   = "NAMEI"
	FENCEFS                 = "FENCEFS"
	FENCEFS_ERR             = FENCEFS + ERR
	LEASESRV                = "LEASESRV"
	MEMFSSRV                = "MEMFSSRV"
	THREADMGR               = "THREADMGR"
	PROTSRV                 = "PROTSRV"
	REFMAP_SUFFIX           = "_REFMAP"
	VERSION                 = "VERSION"
	CLNTCOND                = "CLNTCOND"
	WASMRT                  = "WASMRT"
	WASMRT_ERR              = WASMRT + ERR
)

// 9P
const (
	NPCODEC Tselector = "NPCODEC"
)

// SigmaP
const (
	SPCODEC Tselector = "SPCODEC"
)

// Transport
const (
	FRAME Tselector = "FRAME"
)

// Simulator
const (
	SIM_CLNT         Tselector = "SIM_CLNT"
	SIM_TEST         Tselector = "SIM_TEST"
	SIM_SVC          Tselector = "SIM_SVC"
	SIM_QMGR         Tselector = "SIM_QMGR"
	SIM_QMGR_TIMEOUT Tselector = "SIM_QMGR_TIMEOUT"
	SIM_LB           Tselector = "SIM_LB"
	SIM_LB_SHARD     Tselector = "SIM_LB_SHARD"
	SIM_LB_PROBE     Tselector = "SIM_LB_PROBE"
	SIM_RAW_LAT      Tselector = "SIM_RAW_LAT"
	SIM_LAT_STATS    Tselector = "SIM_LAT_STATS"
	SIM_UTIL_STATS   Tselector = "SIM_UTIL_STATS"
	SIM_RAW_UTIL     Tselector = "SIM_RAW_UTIL"
	SIM_AUTOSCALE    Tselector = "SIM_AUTOSCALE"
)
