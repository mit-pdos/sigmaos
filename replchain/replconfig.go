package replchain

import (
	"fmt"

	"ulambda/repl"
)

type NetServerReplConfig struct {
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

func MakeNetServerReplConfig() *NetServerReplConfig {
	return &NetServerReplConfig{}
}

func CopyNetServerReplConfig(old *NetServerReplConfig) *NetServerReplConfig {
	return &NetServerReplConfig{
		old.LogOps,
		old.ConfigPath,
		old.UnionDirPath,
		old.SymlinkPath,
		old.RelayAddr,
		"", "", "", "", "",
	}
}

func (c *NetServerReplConfig) MakeServer() repl.Server {
	return MakeReplState(c)
}

func (c *NetServerReplConfig) ReplAddr() string {
	return c.RelayAddr
}

func (c *NetServerReplConfig) String() string {
	return fmt.Sprintf("{ relayAddr: %v head: %v tail: %v prev: %v next: %v }", c.RelayAddr, c.HeadAddr, c.TailAddr, c.PrevAddr, c.NextAddr)
}
