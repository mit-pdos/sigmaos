package leadertest

import (
	"sigmaos/proc"
	"sigmaos/sessp"
)

type Config struct {
	Epoch  sessp.Tepoch
	Leader proc.Tpid
	Pid    proc.Tpid
}
