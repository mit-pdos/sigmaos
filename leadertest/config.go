package leadertest

import (
	"sigmaos/proc"
)

type Config struct {
	Epoch  uint64
	Leader proc.Tpid
	Pid    proc.Tpid
}
