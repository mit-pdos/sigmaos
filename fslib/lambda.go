package fslib

import (
	"math/rand"
	"strconv"

	np "ulambda/ninep"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}
