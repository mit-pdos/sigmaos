package sigmap

import (
	"sigmaos/util/rand"
)

func GenPid(program string) Tpid {
	return Tpid(program + "-" + rand.String(16))
}

func (pid Tpid) String() string {
	return string(pid)
}
