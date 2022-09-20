package repldummy

import (
	"fmt"

	"sigmaos/repl"
	"sigmaos/threadmgr"
)

type DummyConfig struct {
}

func MakeConfig() *DummyConfig {
	conf := &DummyConfig{}
	return conf
}

func (rc *DummyConfig) MakeServer(tm *threadmgr.ThreadMgr) repl.Server {
	return MakeDummyReplServer(tm)
}

func (rc *DummyConfig) ReplAddr() string {
	return "dummy-repl-addr"
}

func (rc *DummyConfig) String() string {
	return fmt.Sprintf("&{ dummy-config }")
}
