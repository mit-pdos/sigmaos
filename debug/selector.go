package debug

type Tselector string

// ALWAYS
const (
	ALWAYS Tselector = "ALWAYS"
	ERROR            = "ERROR"
	NEVER            = "NEVER"
)

// ERR
const (
	ERR Tselector = "_ERR"
)

// Benchmarks
const (
	LOADGEN    Tselector = "LOADGEN"
	BENCH                = "BENCH"
	THROUGHPUT           = "THROUGHPUT"
	CPU_UTIL             = "CPU_UTIL"
)

// Latency break-down.
const (
	SPAWN_LAT      Tselector = "SPAWN_LAT"
	NET_LAT                  = "NET_LAT"
	DIALPROXY_LAT            = "DIALPROXY_LAT"
	REALM_GROW_LAT           = "REALM_GROW_LAT"
	CACHE_LAT                = "CACHE_LAT"
	WALK_LAT                 = "WALK_LAT"
	FSETCD_LAT               = "FSETCD_LAT"
	ATTACH_LAT               = "ATTACH_LAT"
	RPC_LAT                  = "RPC_LAT"
)

// Tests
const (
	TEST     Tselector = "TEST"
	TEST1              = "TEST1"
	STAT               = "STAT"
	TEST_LAT           = "TEST_LAT"
	DELAY              = "DELAY"
	CRASH              = "CRASH"
	PERF               = "PERF"
)

// Apps
const (
	WWW                     Tselector = "WWW"
	WWW_ERR                           = WWW + ERR
	WWW_CLNT                          = WWW + "_CLNT"
	MATMUL                            = "MATMUL"
	CACHESRV                          = "CACHESRV"
	REPLSRV                           = "REPLSRV"
	CACHECLERK                        = "CACHECLERK"
	CACHEDSVCCLNT                     = "CACHEDSVCCLNT"
	RPC_BENCH_SRV                     = "RPC_BENCH_SRV"
	RPC_BENCH_CLNT                    = "RPC_BENCH_CLNT"
	HOTEL_CLNT                        = "HOTEL_CLNT"
	HOTEL_GEO                         = "HOTEL_GEO"
	HOTEL_GEO_ERR                     = "HOTEL_GEO" + ERR
	HOTEL_PROF                        = "HOTEL_PROF"
	HOTEL_RATE                        = "HOTEL_RATE"
	HOTEL_RESERVE                     = "HOTEL_RESERVE"
	HOTEL_SEARCH                      = "HOTEL_SEARCH"
	HOTEL_WWW                         = "HOTEL_WWW"
	HOTEL_WWW_ERR                     = "HOTEL_WWW" + ERR
	HOTEL_WWW_STATS                   = "HOTEL_WWW_STATS"
	SLEEPER                           = "SLEEPER"
	SPINNER                           = "SPINNER"
	FSREADER                          = "FSREADER"
	SLEEPER_TIMING                    = "SLEEPER_TIMING"
	IMGD                              = "IMGD"
	IMGD_ERR                          = "IMGD" + ERR
	MR                                = "MR"
	MR_TPT                            = "MR_TPT"
	KVBAL                             = "KVBAL"
	KVBAL_ERR                         = KVBAL + ERR
	KVCLERK                           = "KVCLERK"
	KVCLERK_ERR                       = KVCLERK + ERR
	KVMON                             = "KVMON"
	KVMV                              = "KVMV"
	KVMV_ERR                          = KVMV + ERR
	SOCIAL_NETWORK                    = "SOCIAL_NETWORK"
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

// System
const (
	SYSTEM Tselector = "SYSTEM"
)

// Kernel
const (
	KERNEL          Tselector = "KERNEL"
	KERNELCLNT                = "KERNELCLNT"
	KERNELCLNT_ERR            = "KERNELCLNT_ERR"
	BOOTCLNT                  = "BOOTCLNT"
	BOOT                      = "BOOT"
	CONTAINER                 = "CONTAINER"
	NAMED                     = "NAMED"
	FSETCD                    = "FSETCD"
	PROCMGR                   = "PROCMGR"
	UPROCDMGR                 = "UPROCDMGR"
	UPROCDMGR_ERR             = "UPROCDMGR" + ERR
	UPROCD                    = "UPROCD"
	UPROCD_ERR                = "UPROCD" + ERR
	LCSCHEDCLNT               = "LCSCHEDCLNT"
	LCSCHEDCLNT_ERR           = "LCSCHEDCLNT" + ERR
	LCSCHED                   = "LCSCHED"
	LCSCHED_ERR               = "LCSCHED" + ERR
	BESCHED                   = "BESCHED"
	BESCHED_ERR               = "BESCHED" + ERR
	BESCHEDCLNT               = "BESCHEDCLNT"
	BESCHEDCLNT_ERR           = "BESCHEDCLNT" + ERR
	KEYCLNT                   = "KEYCLNT"
	KEYCLNT_ERR               = "KEYCLNT" + ERR
	KEYD                      = "KEYD"
	KEYD_ERR                  = "KEYD" + ERR
	MSCHED                    = "MSCHED"
	MSCHED_ERR                = "MSCHED" + ERR
	MSCHEDCLNT                = "MSCHEDCLNT"
	MSCHEDCLNT_ERR            = "MSCHEDCLNT" + ERR
	PROCMGR_ERR               = PROCMGR + ERR
	PROCCACHE                 = "PROCCACHE"
	PROCFS                    = "PROCFS"
	CGROUP                    = "CGROUP"
	CGROUP_ERR                = "CGROUP" + ERR
	S3                        = "S3"
	UX                        = "UX"
	DB                        = "DB"
	MONGO                     = "MONGO"
	MONGO_ERR                 = "MONGO" + ERR
	NPPROXY                   = "NPPROXY"
	SPPROXYSRV                = "SPPROXYSRV"
	SPPROXYSRV_ERR            = "SPPROXYSRV" + ERR
	BINSRV                    = "BINSRV"
	CHUNKSRV                  = "CHUNKSRV"
	CHUNKSRV_ERR              = "CHUNKSRV" + ERR
	CHUNKCLNT                 = "CHUNKCLNT"
	CHUNKCLNT_ERR             = "CHUNKCLNT" + ERR
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
	SIGMACLNT              = "SIGMACLNT"
	NODED                  = "NODED"
	NODED_ERR              = NODED + ERR
	MACHINED               = "MACHINED"
	REALM_LOCK             = "REALM_LOCK"
	PORT                   = "PORT"
)

// Client Libraries
const (
	WRITER_ERR    Tselector = "WRITER" + ERR
	READER_ERR              = "READER" + ERR
	AWRITER                 = "AWRITER"
	PREADER                 = "PREADER"
	FSLIB                   = "FSLIB"
	FSLIB_ERR               = "FSLIB" + ERR
	SEMCLNT                 = "SEMCLNT"
	SEMCLNT_ERR             = SEMCLNT + ERR
	EPOCHCLNT               = "EPOCHCLNT"
	EPOCHCLNT_ERR           = EPOCHCLNT + ERR
	FTTASKS                 = "FTTASKS"
	FTTASKMGR               = "FTTASKMGR"
	LEADER                  = "LEADER"
	LEADER_ERR              = LEADER + ERR
	GROUPMGR                = "GROUPMGR"
	GROUPMGR_ERR            = GROUPMGR + ERR
	PROCCLNT                = "PROCCLNT"
	PROCCLNT_ERR            = PROCCLNT + ERR
	FENCECLNT               = "FENCECLNT"
	FENCECLNT_ERR           = FENCECLNT + ERR
	LEASECLNT               = "LEASECLNT"
	ELECTCLNT               = "ELECTCLNT"
	KVGRP                   = "KVGRP"
	KVGRP_ERR               = KVGRP + ERR
	SESSDEVCLNT             = "SESSDEVCLNT"
	K8S_UTIL                = "K8S_UTIL"
	SIGMACLNTCLNT           = "SIGMACLNTCLNT"
	S3CLNT                  = "S3CLNT"
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
	DIALPROXY_ERR              = "DIALPROXY_ERR"
	DIALPROXYSRV               = "DIALPROXYSRV"
	DIALPROXYSRV_ERR           = "DIALPROXYSRV" + ERR
	PYPROXYSRV                 = "PYPROXYSRV"
	PYPROXYSRV_ERR             = "PYPROXYSRV" + ERR
)

// Client-side Infrastructure
const (
	NETCLNT            Tselector = "NETCLNT"
	NETCLNT_ERR                  = NETCLNT + ERR
	DIALPROXYCLNT                = "DIALPROXYCLNT"
	DIALPROXYCLNT_ERR            = "DIALPROXYCLNT" + ERR
	DIALPROXYTRANS               = "DIALPROXYTRANS"
	DIALPROXYTRANS_ERR           = "DIALPROXYTRANS" + ERR
	DEMUXCLNT                    = "DEMUXCLNT"
	DEMUXCLNT_ERR                = "DEMUXCLNT" + ERR
	PROTCLNT                     = "PROTCLNT"
	PROTCLNT_ERR                 = "PROTCLNT" + ERR
	SESS_CLNT_Q                  = "SESS_CLNT_Q"
	SESSCLNT                     = "SESSCLNT"
	SESSCLNT_ERR                 = SESSCLNT + ERR
	FIDCLNT                      = "FIDCLNT"
	FIDCLNT_ERR                  = FIDCLNT + ERR
	RPCCLNT                      = "RPCCLNT"
	MOUNT                        = "MOUNT"
	MOUNT_ERR                    = MOUNT + ERR
	FDCLNT                       = "FDCLNT"
	PATHCLNT                     = "PATHCLNT"
	PATHCLNT_ERR                 = PATHCLNT + ERR
	WALK                         = "WALK"
	WALK_ERR                     = "WALK" + ERR
)

// Server-side Infrastructure
const (
	AUTH          Tselector = "AUTH"
	AUTH_ERR                = AUTH + ERR
	NETSRV                  = "NETSRV"
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
	SIM_CLNT      Tselector = "SIM_CLNT"
	SIM_TEST                = "SIM_TEST"
	SIM_SVC                 = "SIM_SVC"
	SIM_QMGR                = "SIM_QMGR"
	SIM_LB                  = "SIM_LB"
	SIM_RAW_LAT             = "SIM_RAW_LAT"
	SIM_LAT_STATS           = "SIM_LAT_STATS"
	SIM_AUTOSCALE           = "SIM_AUTOSCALE"
)
