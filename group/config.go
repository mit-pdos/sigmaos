package group

import (
	"fmt"

	sp "sigmaos/sigmap"
)

type GroupConfig struct {
	SigmaAddrs []sp.Taddrs
	RaftAddrs  []string
}

func (cfg *GroupConfig) String() string {
	return fmt.Sprintf("&{ SigmaAddrs:%v RaftAddrs:%v }", cfg.SigmaAddrs, cfg.RaftAddrs)
}
