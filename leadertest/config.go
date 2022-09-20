package leadertest

import (
	"sigmaos/proc"
)

type Config struct {
	Epoch  string
	Leader proc.Tpid
	Pid    proc.Tpid
}
