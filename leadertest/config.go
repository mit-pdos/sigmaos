package leadertest

import (
	"fmt"

	sp "sigmaos/sigmap"
)

type Config struct {
	Epoch  sp.Tepoch
	Leader sp.Tpid
	Pid    sp.Tpid
}

func (c *Config) String() string {
	return fmt.Sprintf("&{ epoch:%v leader:%v pid:%v }", c.Epoch, c.Leader, c.Pid)
}
