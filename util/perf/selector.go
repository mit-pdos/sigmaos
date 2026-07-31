package perf

type Tselector string

func (t Tselector) String() string {
	return string(t)
}

// Suffixes
const (
	PPROF       Tselector = "_PPROF"
	PPROF_MEM             = "_PPROF_MEM"
	PPROF_MUTEX           = "_PPROF_MUTEX"
	PPROF_BLOCK           = "_PPROF_BLOCK"
	CPU                   = "_CPU"
	TPT                   = "_TPT"
	VAL                   = "_VAL"
)

// Tests & benchmarking
const (
	TEST  Tselector = "TEST"
	BENCH           = "BENCH"
	COST            = "COST"
)

// Instrumentation which is off by default because it costs something on the
// path it measures.
const (
	// CPU_PHASE_BREAKDOWN turns on per-proc CPU accounting: the CPUPhases
	// chain, the exec -> main and whole-lifetime rusage reports, and
	// LogCPUSince. Each mark is a getrusage plus (under db.SPAWN_LAT) two log
	// lines, on the setup path of every proc — which is exactly the path being
	// measured, so it is opt-in rather than always on.
	CPU_PHASE_BREAKDOWN Tselector = "CPU_PHASE_BREAKDOWN"
)

// kernel procs
const (
	NAMED      Tselector = "NAMED"
	KNAMED               = "KNAMED"
	PROCD                = "PROCD"
	S3                   = "S3"
	UX                   = "UX"
	MSCHED               = "MSCHED"
	KEYD                 = "KEYD"
	SPPROXYSRV           = "SPPROXYSRV"
	BESCHED              = "BESCHED"
	LCSCHED              = "LCSCHED"
)

// libs
const (
	GROUP Tselector = "GROUP"
)

// mr
const (
	MRMAPPER  Tselector = "MRMAPPER"
	MRREDUCER           = "MRREDUCER"
	MRCOORD             = "MRCOORD"
	SEQGREP             = "SEQGREP"
	SEQWC               = "SEQWC"
)

// imgresize
const (
	THUMBNAIL Tselector = "THUMBNAIL"
)

// kv
const (
	KVCLERK Tselector = "KVCLERK"
)

// epcache
const (
	EPCACHE Tselector = "EPCACHE"
)

// hotel
const (
	HOTEL_WWW     Tselector = "HOTEL_WWW"
	HOTEL_GEO               = "HOTEL_GEO"
	HOTEL_RESERVE           = "HOTEL_RESERVE"
	HOTEL_SEARCH            = "HOTEL_SEARCH"
	HOTEL_MATCH             = "HOTEL_MATCH"
	HOTEL_RATE              = "HOTEL_RATE"
)

// socialnetwork
const (
	SOCIAL_NETWORK_FRONTEND Tselector = "SOCIAL_NETWORK_FRONTEND"
	SOCIAL_NETWORK_USER               = "SOCIAL_NETWORK_USER"
	SOCIAL_NETWORK_GRAPH              = "SOCIAL_NETWORK_GRAPH"
	SOCIAL_NETWORK_POST               = "SOCIAL_NETWORK_POST"
	SOCIAL_NETWORK_TIMELINE           = "SOCIAL_NETWORK_TIMELINE"
	SOCIAL_NETWORK_HOME               = "SOCIAL_NETWORK_HOME"
	SOCIAL_NETWORK_COMPOSE            = "SOCIAL_NETWORK_COMPOSE"
)

// cache
const (
	CACHECLERK Tselector = "CACHECLERK"
	CACHESRV             = "CACHESRV"
)

// microbenchmarks
const (
	WRITER         Tselector = "WRITER"
	BUFWRITER                = "BUFWRITER"
	ABUFWRITER               = "ABUFWRITER"
	READER                   = "READER"
	BUFREADER                = "BUFREADER"
	ABUFREADER               = "ABUFREADER"
	RPC_BENCH_SRV            = "RPC_BENCH_SRV"
	RPC_BENCH_CLNT           = "RPC_BENCH_CLNT"
)
