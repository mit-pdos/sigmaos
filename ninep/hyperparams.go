package ninep

import (
	"log"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var Target = "local"

// Local params
var local = `
conn:
  msg_len: 65536

perf:
  cpu_util_sample_hz: 50

session:
  heartbeat_interval: 50ms
  timeout: 1000ms

realm:
  scan_interval: 50ms 
  resize_interval: 100ms
  grow_cpu_util_threshold: 50
  shrink_cpu_util_threshold: 25

machine:
  core_group_fraction: 0.5

procd:
  stealable_proc_timeout: 100ms
  work_steal_scan_timeout: 100ms
  be_proc_claim_cpu_threshold: 90.0
  be_proc_oversubscription_rate: 1.0

raft:
  tick_interval: 25ms
  elect_nticks: 4
  heartbeat_ticks: 1
 `

// AWS params
var aws = `
conn:
  msg_len: 65536

perf:
  cpu_util_sample_hz: 50

session:
  heartbeat_interval: 1000ms
  timeout: 40000ms

realm:
  scan_interval: 1000ms
  resize_interval: 1000ms
  grow_cpu_util_threshold: 50
  shrink_cpu_util_threshold: 25

machine:
  core_group_fraction: 0.5

procd:
  stealable_proc_timeout: 1000ms
  work_steal_scan_timeout: 1000ms
  be_proc_claim_cpu_threshold: 90.0
  be_proc_oversubscription_rate: 1.0

raft:
  tick_interval: 500ms
  elect_nticks: 4 
  heartbeat_ticks: 1
 `

type Config struct {
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
		// Frequency with which realmmgr scans to rebalance realms.
		SCAN_INTERVAL time.Duration `yaml:"scan_interval"`
		// Maximum frequency with which realmmgr resizes a realm.
		RESIZE_INTERVAL time.Duration `yaml:"resize_interval"`
		// Utilization threshold at which to grow a realm.
		GROW_CPU_UTIL_THRESHOLD float64 `yaml:"grow_cpu_util_threshold"`
		// Utilization threshold at which to shrink a realm.
		SHRINK_CPU_UTIL_THRESHOLD float64 `yaml:"shrink_cpu_util_threshold"`
	} `yaml:"realm"`
	Machine struct {
		// Core group size, in terms of fractions of a machine.
		CORE_GROUP_FRACTION float64 `yaml:"core_group_fraction"`
	} `yaml:"machine"`
	Procd struct {
		// Procd work steal frequency.
		STEALABLE_PROC_TIMEOUT  time.Duration `yaml:"stealable_proc_timeout"`
		WORK_STEAL_SCAN_TIMEOUT time.Duration `yaml:"work_steal_scan_timeout"`
		// CPU utilization threshold at which procd will no longer pull & run BE
		// procs.
		BE_PROC_CLAIM_CPU_THRESHOLD float64 `yaml:"be_proc_claim_cpu_threshold"`
		// Max oversubscription factor per claim interval when deciding whether or
		// not to claim a be proc.
		BE_PROC_OVERSUBSCRIPTION_RATE float64 `yaml:"be_proc_oversubscription_rate"`
	} `yaml:"procd"`
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
	case "aws":
		Conf = ReadConfig(aws)
	default:
		Conf = ReadConfig(local)
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
