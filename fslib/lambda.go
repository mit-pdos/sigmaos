package fslib

import (
	"math/rand"
	"strconv"
)

const (
	PROCD_ROOT = "name/procds"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}
