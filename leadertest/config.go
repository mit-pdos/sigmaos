package leadertest

import (
	"ulambda/proc"
)

type Config struct {
	Epoch  string
	Leader proc.Tpid
	Pid    proc.Tpid
}
