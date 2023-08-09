package debug

type Tselector string

// ALWAYS
const (
	ALWAYS Tselector = "ALWAYS"
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
	SPAWN_LAT Tselector = "SPAWN_LAT"
	SESS_LAT            = "SESS_LAT"
	CACHE_LAT           = "CACHE_LAT"
)

// Tests
const (
	TEST  Tselector = "TEST"
	TEST1           = "TEST1"
	DELAY           = "DELAY"
	CRASH           = "CRASH"
)

// Apps
const (
	WWW                     Tselector = "WWW"
	WWW_ERR                           = WWW + ERR
	WWW_CLNT                          = WWW + "_CLNT"
	MATMUL                            = "MATMUL"
	CACHESRV                          = "CACHESRV"
	CACHESRV_REPL                     = "CACHESRV_REPL"
	CACHECLERK                        = "CACHECLERK"
	CACHEDSVCCLNT                     = "CACHEDSVCCLNT"
	RPC_BENCH_SRV                     = "RPC_BENCH_SRV"
	RPC_BENCH_CLNT                    = "RPC_BENCH_CLNT"
	HOTEL_CLNT                        = "HOTEL_CLNT"
	HOTEL_GEO                         = "HOTEL_GEO"
	HOTEL_PROF                        = "HOTEL_PROF"
	HOTEL_RATE                        = "HOTEL_RATE"
	HOTEL_RESERVE                     = "HOTEL_RESERVE"
	HOTEL_SEARCH                      = "HOTEL_SEARCH"
	HOTEL_WWW                         = "HOTEL_WWW"
	HOTEL_WWW_STATS                   = "HOTEL_WWW_STATS"
	SLEEPER                           = "SLEEPER"
	SPINNER                           = "SPINNER"
	FSREADER                          = "FSREADER"
	SLEEPER_TIMING                    = "SLEEPER_TIMING"
	IMGD                              = "IMGD"
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
	KERNEL         Tselector = "KERNEL"
	BOOTCLNT                 = "BOOTCLNT"
	BOOT                     = "BOOT"
	CONTAINER                = "CONTAINER"
	NAMED                    = "NAMED"
	FSETCD                   = "FSETCD"
	FSETCDLEASE              = "FSETCDLEASE"
	PROCMGR                  = "PROCMGR"
	UPROCDMGR                = "UPROCDMGR"
	UPROCD                   = "UPROCD"
	UPROCD_ERR               = "UPROCD" + ERR
	SCHEDD                   = "SCHEDD"
	SCHEDD_ERR               = "SCHEDD" + ERR
	SCHEDDCLNT               = "SCHEDDCLNT"
	SCHEDDCLNT_ERR           = "SCHEDDCLNT" + ERR
	PROCMGR_ERR              = PROCMGR + ERR
	PROCCACHE                = "PROCCACHE"
	CGROUP                   = "CGROUP"
	CGROUP_ERR               = "CGROUP_ERR"
	S3                       = "S3"
	UX                       = "UX"
	DB                       = "DB"
	MONGO                    = "MONGO"
	PROXY                    = "PROXY"
)

// Realm
const (
	SIGMAMGR     Tselector = "SIGMAMGR"
	SIGMAMGR_ERR           = SIGMAMGR + ERR
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
	FSLIB                   = "FSLIB"
	SEMCLNT                 = "SEMCLNT"
	SEMCLNT_ERR             = SEMCLNT + ERR
	EPOCHCLNT               = "EPOCHCLNT"
	EPOCHCLNT_ERR           = EPOCHCLNT + ERR
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
)

// Server Libraries
const (
	MEMFS      Tselector = "MEMFS"
	PIPE                 = "PIPE"
	OVERLAYDIR           = "OVERLAYDIR"
	CLONEDEV             = "CLONEDEV"
	SESSDEV              = "SESSDEV"
	SIGMASRV             = "SIGMASRV"
)

// Client-side Infrastructure
const (
	NETCLNT             Tselector = "NETCLNT"
	NETCLNT_ERR                   = NETCLNT + ERR
	SESS_CLNT_Q                   = "SESS_CLNT_Q"
	SESS_STATE_CLNT               = "SESS_STATE_CLNT"
	SESS_STATE_CLNT_ERR           = SESS_STATE_CLNT + ERR
	FDCLNT                        = "FDCLNT"
	FDCLNT_ERR                    = FDCLNT + ERR
	FIDCLNT                       = "FIDCLNT"
	FIDCLNT_ERR                   = FIDCLNT + ERR
	MOUNT                         = "MOUNT"
	PATHCLNT                      = "PATHCLNT"
	PATHCLNT_ERR                  = PATHCLNT + ERR
	WALK                          = "WALK"
	SVCMOUNT                      = "SVCMOUNT"
)

// Server-side Infrastructure
const (
	NETSRV             Tselector = "NETSRV"
	NETSRV_ERR                   = NETSRV + ERR
	REPLRAFT                     = "REPLRAFT"
	RAFT_TIMING                  = "RAFT_TIMING"
	REPLY_TABLE                  = "REPLY_TABLE"
	INTERVALS                    = "INTERVALS"
	SESSSRV                      = "SESSSRV"
	WATCH                        = "WATCH"
	WATCH_ERR                    = WATCH + ERR
	LOCKMAP                      = "LOCKMAP"
	SNAP                         = "SNAP"
	NAMEI                        = "NAMEI"
	FENCESRV                     = "FENCESRV"
	FENCEFS                      = "FENCEFS"
	FENCEFS_ERR                  = FENCEFS + ERR
	LEASESRV                     = "LEASESRV"
	MEMFSSRV                     = "MEMFSSRV"
	THREADMGR                    = "THREADMGR"
	PROTSRV                      = "PROTSRV"
	REFMAP_SUFFIX                = "_REFMAP"
	VERSION                      = "VERSION"
	SESSCOND                     = "SESSCOND"
	SESS_STATE_SRV               = "SESS_STATE_SRV"
	SESS_STATE_SRV_ERR           = SESS_STATE_SRV + ERR
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
