package debug

type Tselector int

//type Tselectorstr string

// ALWAYS
const (
	ALWAYS Tselector = 0
)

// ERR
const (
	ERR Tselector = 0
)

// Benchmarks
const (
	LOADGEN Tselector = 0
	BENCH             = 0
)

// Tests
const (
	TEST  Tselector = 0 //"TEST"
	TEST1           = 0
	DELAY           = 0
)

// Apps
const (
	WWW             Tselector = 0 //"WWW"
	WWW_ERR                   = 0
	WWW_CLNT                  = 0
	MATMUL                    = 0
	CACHESRV                  = 0
	CACHECLERK                = 0
	HOTEL_CLNT                = 0
	HOTEL_GEO                 = 0
	HOTEL_PROF                = 0
	HOTEL_RATE                = 0
	HOTEL_RESERVE             = 0
	HOTEL_SEARCH              = 0
	HOTEL_WWW                 = 0
	HOTEL_WWW_STATS           = 0
	SLEEPER                   = 0
	SPINNER                   = 0
	FSREADER                  = 0
	SLEEPER_TIMING            = 0
	MR                        = 0
	MR_TPT                    = 0
	KVBAL                     = 0
	KVBAL_ERR                 = 0
	KVCLERK                   = 0
	KVCLERK_ERR               = 0
	KVMON                     = 0
	KVMV                      = 0
	KVMV_ERR                  = 0
)

// Kernel
const (
	KERNEL     Tselector = 0
	NAMED                = 0
	PROCD                = 0
	PROCD_ERR            = 0
	PROCD_PERF           = 0
	PROCCACHE            = 0
	S3                   = 0
	UX                   = 0
	DB                   = 0
	PROXY                = 0
)

// Realm
const (
	SIGMAMGR     Tselector = 0 //"SIGMAMGR"
	SIGMAMGR_ERR           = 0 //"SIGMAMGR_ERR"
	REALMMGR               = 0
	REALMMGR_ERR           = 0
	REALMCLNT              = 0
	NODED                  = 0
	NODED_ERR              = 0
	MACHINED               = 0
	REALM_LOCK             = 0
)

// Client Libraries
const (
	WRITER_ERR    Tselector = 0
	READER_ERR              = 0
	AWRITER                 = 0
	FDCLNT_ERR              = 0
	FSLIB                   = 0
	SEMCLNT                 = 0
	SEMCLNT_ERR             = 0
	EPOCHCLNT               = 0
	EPOCHCLNT_ERR           = 0
	LEADER_ERR              = 0
	GROUPMGR                = 0
	GROUPMGR_ERR            = 0
	PROCCLNT                = 0
	PROCCLNT_ERR            = 0
	FENCECLNT               = 0
	FENCECLNT_ERR           = 0
	GROUP                   = 0
	GROUP_ERR               = 0
)

// Server Libraries
const (
	MEMFS      Tselector = 0
	PIPE                 = 0
	OVERLAYDIR           = 0
	CLONEDEV             = 0
	SESSDEV              = 0
	PROTDEVSRV           = 0
)

// Client-side Infrastructure
const (
	NETCLNT             Tselector = 0
	NETCLNT_ERR                   = 0
	SESS_CLNT_Q                   = 0
	SESS_STATE_CLNT               = 0
	SESS_STATE_CLNT_ERR           = 0
	FIDCLNT                       = 0
	MOUNT                         = 0
	PATHCLNT                      = 0
	PATHCLNT_ERR                  = 0
	WALK                          = 0
)

// Server-side Infrastructure
const (
	NETSRV             Tselector = 0 //"_REFMAP"
	NETSRV_ERR                   = 0
	REPLRAFT                     = 0
	REPLY_TABLE                  = 0
	SESSSRV                      = 0
	WATCH                        = 0
	WATCH_ERR                    = 0
	RAFT_TIMING                  = 0
	LOCKMAP                      = 0
	SNAP                         = 0
	NAMEI                        = 0
	FENCE_SRV                    = 0
	FENCEFS                      = 0
	FENCEFS_ERR                  = 0
	THREADMGR                    = 0
	PROTSRV                      = 0
	REFMAP_SUFFIX                = 0
	VERSION                      = 0
	SESSCOND                     = 0
	SESS_STATE_SRV               = 0
	SESS_STATE_SRV_ERR           = 0
	FRAME                        = 0
)

// 9P
const (
	NPCODEC Tselector = 0
)

// SigmaP
const (
	SPCODEC Tselector = 0
)
