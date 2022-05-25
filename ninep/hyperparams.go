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
session:
  heartbeat_ms: 50ms
  timeout_ms: 200ms

realm:
  scan_interval_ms: 50ms 
  resize_interval_ms: 100ms
  grow_cpu_util_threshold: 50
  shrink_cpu_util_threshold: 25

procd:
  stealable_proc_timeout_ms : 100ms
  work_steal_scan_timeout_ms: 100ms

raft:
  tick_ms         : 25ms
  elect_nticks    : 4
  heartbeat_ticks : 1
 `

// AWS params
var aws = `
session:
  heartbeat_ms: 1000ms
  timeout_ms: 40000ms

realm:
  scan_interval_ms: 1000ms
  resize_interval_ms: 1000ms
  grow_cpu_util_threshold: 50
  shrink_cpu_util_threshold: 25

procd:
  stealable_proc_timeout_ms : 1000ms
  work_steal_scan_timeout_ms: 1000ms

raft:
  tick_ms         : 500ms
  elect_nticks    : 4 
  heartbeat_ticks : 1
 `

type Config struct {
	Session struct {
		// Client heartbeat frequency.
		HEARTBEAT_MS time.Duration `yaml:"heartbeat_ms"`
		// Kill a session after timeout ms of missed heartbeats.
		TIMEOUT_MS time.Duration `yaml:"timeout_ms"`
	} `yaml:"session"`
	Realm struct {
		// Frequency with which realmmgr scans to rebalance realms.
		SCAN_INTERVAL_MS time.Duration `yaml:"scan_interval_ms"`
		// Maximum frequency with which realmmgr resizes a realm.
		RESIZE_INTERVAL_MS time.Duration `yaml:"resize_interval_ms"`
		// Utilization threshold at which to grow a realm.
		GROW_CPU_UTIL_THRESHOLD float64 `yaml:"grow_cpu_util_threshold"`
		// Utilization threshold at which to shrink a realm.
		SHRINK_CPU_UTIL_THRESHOLD float64 `yaml:"shrink_cpu_util_threshold"`
	} `yaml:"realm"`
	Procd struct {
		// Procd work steal frequency.
		STEALABLE_PROC_TIMEOUT_MS  time.Duration `yaml:"stealable_proc_timeout_ms"`
		WORK_STEAL_SCAN_TIMEOUT_MS time.Duration `yaml:"work_steal_scan_timeout_ms"`
	} `yaml:"procd"`
	Raft struct {
		// Frequency with which the raft library ticks
		TICK_MS time.Duration `yaml:"tick_ms"`
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
