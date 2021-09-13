package replchain

import (
	"fmt"

	"ulambda/repl"
)

type ChainReplConfig struct {
	LogOps       bool
	ConfigPath   string
	UnionDirPath string
	SymlinkPath  string
	RelayAddr    string
	LastSendAddr string
	HeadAddr     string
	TailAddr     string
	PrevAddr     string
	NextAddr     string
}

func MakeChainReplConfig() *ChainReplConfig {
	return &ChainReplConfig{}
}

func CopyChainReplConfig(old *ChainReplConfig) *ChainReplConfig {
	return &ChainReplConfig{
		old.LogOps,
		old.ConfigPath,
		old.UnionDirPath,
		old.SymlinkPath,
		old.RelayAddr,
		"", "", "", "", "",
	}
}

func (c *ChainReplConfig) MakeServer() repl.Server {
	return MakeChainReplServer(c)
}

func (c *ChainReplConfig) ReplAddr() string {
	return c.RelayAddr
}

func (c *ChainReplConfig) String() string {
	return fmt.Sprintf("{ relayAddr: %v head: %v tail: %v prev: %v next: %v }", c.RelayAddr, c.HeadAddr, c.TailAddr, c.PrevAddr, c.NextAddr)
}
