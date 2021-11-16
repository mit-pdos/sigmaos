package proc

import (
	"os"

	"github.com/thanhpk/randstr"
)

func GenPid() string {
	return randstr.Hex(8)
}

// Can return "" for test programs that make a procclnt
func GetPid() string {
	return os.Getenv("SIGMAPID")
}

// Can return "" for test programs that make a procclnt
func GetPidDir() string {
	return os.Getenv("SIGMAPIDDIR")
}
