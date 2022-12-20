package machine

import (
	"fmt"

	"sigmaos/sessp"
	"sigmaos/linuxsched"
)

type Config struct {
	Cores *sessp.Tinterval
}

func makeMachineConfig() *Config {
	cfg := MakeEmptyConfig()
	cfg.Cores = sessp.MkInterval(0, uint64(linuxsched.NCores))
	return cfg
}

func MakeEmptyConfig() *Config {
	return &Config{}
}

func (cfg *Config) String() string {
	return fmt.Sprintf("&{ cores:%v }", cfg.Cores)
}
