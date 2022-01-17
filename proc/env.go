package proc

import (
	"os"

	"ulambda/rand"
)

// Environment variables which every proc expects to have.
const (
	SIGMANEWROOT   = "SIGMANEWROOT"
	SIGMAPROCDIP   = "SIGMAPROCDIP"
	SIGMAPID       = "SIGMAPID"
	SIGMAPROCDIR   = "SIGMAPROCDIR"
	SIGMAPARENTDIR = "SIGMAPARENTDIR"
)

func GenPid() string {
	return rand.String(8)
}

func SetPid(pid string) {
	os.Setenv(SIGMAPID, pid)
}

// Can return "" for test programs that make a procclnt
func GetPid() string {
	return os.Getenv(SIGMAPID)
}

func SetProcdIp(procdIp string) {
	os.Setenv(SIGMAPROCDIP, procdIp)
}

func GetProcdIp() string {
	return os.Getenv(SIGMAPROCDIP)
}

func SetProcDir(procdir string) {
	os.Setenv(SIGMAPROCDIR, procdir)
}

func GetProcDir() string {
	return os.Getenv(SIGMAPROCDIR)
}

func SetParentDir(procdir string) {
	os.Setenv(SIGMAPARENTDIR, procdir)
}

func GetParentDir() string {
	return os.Getenv(SIGMAPARENTDIR)
}

func FakeProcEnv(pid, procdip, procdir, parentdir string) {
	SetPid(pid)
	SetProcDir(procdir)
	SetParentDir(parentdir)
}
