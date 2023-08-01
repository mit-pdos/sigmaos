package leadertest

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type Config struct {
	Epoch  sp.Tepoch
	Leader proc.Tpid
	Pid    proc.Tpid
}
