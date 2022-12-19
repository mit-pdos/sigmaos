package machine

import (
	"fmt"

	"sigmaos/linuxsched"
	sp "sigmaos/sigmap"
)

type Config struct {
	Cores *sp.Tinterval
}

func makeMachineConfig() *Config {
	cfg := MakeEmptyConfig()
	cfg.Cores = sp.MkInterval(0, uint64(linuxsched.NCores))
	return cfg
}

func MakeEmptyConfig() *Config {
	return &Config{}
}

func (cfg *Config) String() string {
	return fmt.Sprintf("&{ cores:%v }", cfg.Cores)
}
