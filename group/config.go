package group

import (
	"fmt"
)

type GroupConfig struct {
	SigmaAddrs []string
	RaftAddrs  []string
}

func (cfg *GroupConfig) String() string {
	return fmt.Sprintf("&{ SigmaAddrs:%v RaftAddrs:%v }", cfg.SigmaAddrs, cfg.RaftAddrs)
}
