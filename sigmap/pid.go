package sigmap

import (
	"sigmaos/util/rand"
)

func GenPid(program string) Tpid {
	return Tpid(program + "-" + rand.Name())
}

func (pid Tpid) String() string {
	return string(pid)
}
