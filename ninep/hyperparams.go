package ninep

import (
	"flag"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Session struct {
		HEARTBEAT_MS time.Duration `yaml:"heartbeat_ms"`
		TIMEOUT_MS   time.Duration `yaml:"timeout_ms"`
	} `yaml:"session"`
	Realm struct {
		SCAN_INTERVAL_MS          time.Duration `yaml:"scan_interval_ms"`
		RESIZE_INTERVAL_MS        time.Duration `yaml:"resize_interval_ms"`
		GROW_CPU_UTIL_THRESHOLD   float64       `yaml:"grow_cpu_util_threshold"`
		SHRINK_CPU_UTIL_THRESHOLD float64       `yaml:"shrink_cpu_util_threshold"`
	} `yaml:"realm"`
	Procd struct {
		WORK_STEAL_TIMEOUT_MS time.Duration `yaml:"work_steal_timeout_ms"`
	} `yaml:"procd"`
	Raft struct {
		TICK_MS         time.Duration `yaml:"tick_ms"`
		ELECT_NTICKS    int           `yaml:"elect_nticks"`
		HEARTBEAT_TICKS int           `yaml:"heartbeat_ticks"`
	} `yaml:"raft"`
}

var Conf *Config
var conf string

func init() {
	flag.StringVar(&conf, "conf", "local", "deployment conf")
	Conf = ReadConfig("../hyperparams-" + conf + ".yml")
}

func ReadConfig(fn string) *Config {
	config := &Config{}
	file, err := os.Open(fn)
	if err != nil {
		log.Fatalf("ReadConfig %v err %v\n", fn, err)
	}
	defer file.Close()

	d := yaml.NewDecoder(file)

	if err := d.Decode(&config); err != nil {
		log.Fatalf("Yalm decode %v err %v\n", fn, err)
	}

	return config
}
