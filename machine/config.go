package machine

import (
	"fmt"

	"ulambda/linuxsched"
	np "ulambda/ninep"
)

type Config struct {
	Cores *np.Tinterval
}

func makeMachineConfig() *Config {
	cfg := MakeEmptyConfig()
	cfg.Cores = np.MkInterval(0, np.Toffset(linuxsched.NCores))
	return cfg
}

func MakeEmptyConfig() *Config {
	return &Config{}
}

func (cfg *Config) String() string {
	return fmt.Sprintf("&{ cores:%v }", cfg.Cores)
}
