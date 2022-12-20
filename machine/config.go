package machine

import (
	"fmt"

	"sigmaos/fcall"
	"sigmaos/linuxsched"
)

type Config struct {
	Cores *fcall.Tinterval
}

func makeMachineConfig() *Config {
	cfg := MakeEmptyConfig()
	cfg.Cores = fcall.MkInterval(0, uint64(linuxsched.NCores))
	return cfg
}

func MakeEmptyConfig() *Config {
	return &Config{}
}

func (cfg *Config) String() string {
	return fmt.Sprintf("&{ cores:%v }", cfg.Cores)
}
