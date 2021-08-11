package fslib

import (
	"math/rand"
	"strconv"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}
