package proc

import (
	"os"

	"ulambda/rand"
)

func GenPid() string {
	return rand.String(8)
}

// Can return "" for test programs that make a procclnt
func GetPid() string {
	return os.Getenv("SIGMAPID")
}

// Can return "" for test programs that make a procclnt
func GetPidDir() string {
	return os.Getenv("SIGMAPIDDIR")
}
