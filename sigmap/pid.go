package sigmap

import (
	"sigmaos/rand"
)

func GenPid(program string) Tpid {
	return Tpid(program + "-" + rand.String(8))
}

func (pid Tpid) String() string {
	return string(pid)
}
