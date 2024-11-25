package sigmap

import (
	"log"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var Target = "local"
var Version = "1.0"

// Local params
var local = `
util:
  stats: true

apparmor:
  enabled: false

uprocsrv:
  pool_sz: 2

chunk:
  chunk_sz: 1048576

fslib:
  max_retry: 100

path:
  max_symlink: 8
  path_resolve_timeout: 200ms
  max_resolve_retry: 30

conn:
  msg_len: 65536

perf:
  cpu_util_sample_hz: 50

session:
  heartbeat_interval: 50ms
  timeout: 1000ms

realm:
  refresh_kernel_srv_interval: 100ms
  fairness_check_period: 1s
  n_sample: 2
  starvation_ratio: 0.1

besched:
  get_proc_timeout: 50ms

raft:
  tick_interval: 25ms
  elect_nticks: 4
  heartbeat_ticks: 1
`

// AWS params
var remote = `
util:
  stats: true

apparmor:
  enabled: true

uprocsrv:
  pool_sz: 2

chunk:
  chunk_sz: 1048576

fslib:
  max_retry: 100

path:
  max_symlink: 8
  path_resolve_timeout: 200ms
  max_resolve_retry: 30

conn:
  msg_len: 65536

perf:
  cpu_util_sample_hz: 50

session:
  heartbeat_interval: 1000ms
  timeout: 40000ms

realm:
  refresh_kernel_srv_interval: 100ms
  fairness_check_period: 1s
  n_sample: 2
  starvation_ratio: 0.1

besched:
  get_proc_timeout: 50ms

raft:
  tick_interval: 500ms
  elect_nticks: 4 
  heartbeat_ticks: 1
`

type Config struct {
	Util struct {
		// Record stats
		STATS bool `yaml:"stats"`
	} `yaml:"util"`
	AppArmor struct {
		// SigmaP connection message length.
		ENABLED bool `yaml:"enabled"`
	}
	UProcSrv struct {
		// Size of Uprocsrv pool
		POOL_SZ int `yaml:"pool_sz"`
	} `yaml:"chunk"`
	Chunk struct {
		// Binary chunk size
		CHUNK_SZ int64 `yaml:"chunk_sz"`
	} `yaml:"chunk"`
	FsLib struct {
		// Max number of retries at the FsLib layer
		MAX_RETRY int `yaml:"max_retry"`
	} `yaml:"fslib"`
	Path struct {
		// Max symlink depth allowed
		MAX_SYMLINK int `yaml:"max_symlink"`
		// Timeout for path resolution
		RESOLVE_TIMEOUT time.Duration `yaml:"path_resolve_timeout"`
		// Max number of path resolution retries, in the event of errors
		MAX_RESOLVE_RETRY int `yaml:"max_resolve_retry"`
	} `yaml:"path"`
	Conn struct {
		// SigmaP connection message length.
		MSG_LEN int `yaml:"msg_len"`
	}
	Perf struct {
		// SigmaP connection message length.
		CPU_UTIL_SAMPLE_HZ int `yaml:"cpu_util_sample_hz"`
	}
	Session struct {
		// Client heartbeat frequency.
		HEARTBEAT_INTERVAL time.Duration `yaml:"heartbeat_interval"`
		// Kill a session after timeout ms of missed heartbeats.
		TIMEOUT time.Duration `yaml:"timeout"`
	} `yaml:"session"`
	Realm struct {
		// Maximum frequency with which to refresh kernel servers.
		KERNEL_SRV_REFRESH_INTERVAL time.Duration `yaml:"refresh_kernel_srv_interval"`
		// Period at which realms' utiliztaion statistics are checked for fairness
		FAIRNESS_CHECK_PERIOD time.Duration `yaml:"fairness_check_period"`
		// Number of samples for fairness check
		N_SAMPLE int `yaml:"n_sample"`
		// Maximum starvation ratio before fairness is enforced
		STARVATION_RATIO float64 `yaml:"starvation_ratio"`
	} `yaml:"realm"`
	BESched struct {
		// Timeout for which an msched's request for a proc to a besched shard lasts
		GET_PROC_TIMEOUT time.Duration `yaml:"get_proc_timeout"`
	} `yaml:"besched"`
	Raft struct {
		// Frequency with which the raft library ticks
		TICK_INTERVAL time.Duration `yaml:"tick_interval"`
		// Number of ticks with no leader heartbeat after which a follower starts an election.
		ELECT_NTICKS int `yaml:"elect_nticks"`
		// Number of heartbeats per tick.
		HEARTBEAT_TICKS int `yaml:"heartbeat_ticks"`
	} `yaml:"raft"`
}

var Conf *Config

func init() {
	switch Target {
	case "remote":
		Conf = ReadConfig(remote)
	case "local":
		Conf = ReadConfig(local)
	default:
		log.Fatalf("Built for unknown target %s", Target)
	}
}

func ReadConfig(params string) *Config {
	config := &Config{}
	d := yaml.NewDecoder(strings.NewReader(params))
	if err := d.Decode(&config); err != nil {
		log.Fatalf("Yalm decode %v err %v\n", params, err)
	}

	return config
}
