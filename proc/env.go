package proc

import (
	"os"

	"ulambda/rand"
)

// Environment variables which every proc expects to have.
const (
	SIGMAPRIVILEGEDPROC = "SIGMAKERNELPROC"
	SIGMANEWROOT        = "SIGMANEWROOT"
	SIGMAPROCDIP        = "SIGMAPROCDIP"
	SIGMAPID            = "SIGMAPID"
	SIGMAPROGRAM        = "SIGMAPROGRAM"
	SIGMAPROCDIR        = "SIGMAPROCDIR"
	SIGMAPARENTDIR      = "SIGMAPARENTDIR"
	SIGMACRASH          = "SIGMACRASH"
	SIGMAPARTITION      = "SIGMAPARTITION"
)

func GenPid() Tpid {
	return Tpid(rand.String(8))
}

func SetPid(pid Tpid) {
	os.Setenv(SIGMAPID, string(pid))
}

// Can return "" for test programs that make a procclnt
func GetPid() Tpid {
	return Tpid(os.Getenv(SIGMAPID))
}

func GetProgram() string {
	return os.Getenv(SIGMAPROGRAM)
}

func GetName() string {
	return GetProgram() + "-" + GetPid().String()
}

func SetProgram(program string) {
	os.Setenv(SIGMAPROGRAM, program)
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

func GetNewRoot() string {
	return os.Getenv(SIGMANEWROOT)
}

func GetIsPrivilegedProc() bool {
	return os.Getenv(SIGMAPRIVILEGEDPROC) == "true"
}

func FakeProcEnv(pid Tpid, program, procdIp, procdir, parentdir string) {
	SetPid(pid)
	SetProgram(program)
	SetProcdIp(procdIp)
	SetProcDir(procdir)
	SetParentDir(parentdir)
}
