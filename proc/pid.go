package proc

import (
	"os"

	"math/rand"
	"strconv"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}

// Can return "" for test programs that make a procclnt
func GetPid() string {
	return os.Getenv("SIGMAPID")
}

// Can return "" for test programs that make a procclnt
func GetPidDir() string {
	return os.Getenv("SIGMAPIDDIR")
}
