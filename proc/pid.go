package proc

import (
	"os"

	"ulambda/rand"
)

func GenPid() string {
	return rand.String(8)
}

func SetPid(pid string) {
	os.Setenv("SIGMAPID", pid)
}

// Can return "" for test programs that make a procclnt
func GetPid() string {
	return os.Getenv("SIGMAPID")
}

// Can return "" for test programs that make a procclnt
func GetPidDir() string {
	return os.Getenv("SIGMAPIDDIR")
}

// Can return "" for test programs that make a procclnt
func GetParentPid() string {
	return os.Getenv("SIGMAPARENTPID")
}

// Can return "" for test programs that make a procclnt
func GetParentPidDir() string {
	return os.Getenv("SIGMAPARENTPIDDIR")
}

// Can return "" for test programs that make a procclnt
func GetProcdIP() string {
	return os.Getenv("SIGMAPROCDIP")
}
