package fslib

import (
	"math/rand"
	"strconv"
)

const (
	PROCD_ROOT   = "name/procds"
	NO_OP_LAMBDA = "no-op-lambda"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}
