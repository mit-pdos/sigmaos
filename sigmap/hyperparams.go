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
apparmor:
  enabled: false

conn:
  msg_len: 65536

perf:
  cpu_util_sample_hz: 50

session:
  heartbeat_interval: 50ms
  timeout: 1000ms

realm:
  refresh_kernel_srv_interval: 100ms

schedd:
  stealable_proc_timeout: 100ms
  work_steal_scan_timeout: 100ms

raft:
  tick_interval: 25ms
  elect_nticks: 4
  heartbeat_ticks: 1
`

// AWS params
var aws = `
apparmor:
  enabled: true

conn:
  msg_len: 65536

perf:
  cpu_util_sample_hz: 50

session:
  heartbeat_interval: 1000ms
  timeout: 40000ms

realm:
  refresh_kernel_srv_interval: 100ms

schedd:
  stealable_proc_timeout: 50ms
  work_steal_scan_timeout: 50ms

raft:
  tick_interval: 500ms
  elect_nticks: 4 
  heartbeat_ticks: 1
`

type Config struct {
	AppArmor struct {
		// SigmaP connection message length.
		ENABLED bool `yaml:"enabled"`
	}
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
	} `yaml:"realm"`
	Schedd struct {
		// Time a proc remains un-spawned before becoming stealable.
		STEALABLE_PROC_TIMEOUT time.Duration `yaml:"stealable_proc_timeout"`
		// Frequency with which schedds scan the ws queue.
		WORK_STEAL_SCAN_TIMEOUT time.Duration `yaml:"work_steal_scan_timeout"`
	} `yaml:"schedd"`
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
