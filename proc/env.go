package proc

import (
	"os"
	"strings"

	"sigmaos/rand"
)

// Environment variables which every proc expects to have.
const (
	SIGMAPRIVILEGEDPROC = "SIGMAKERNELPROC"
	SIGMAPROCDIP        = "SIGMAPROCDIP"
	SIGMANODEDID        = "SIGMANODEDID"
	SIGMAPID            = "SIGMAPID"
	SIGMAPROGRAM        = "SIGMAPROGRAM"
	SIGMAPROCDIR        = "SIGMAPROCDIR"
	SIGMAPARENTDIR      = "SIGMAPARENTDIR"
	SIGMACRASH          = "SIGMACRASH"
	SIGMAPARTITION      = "SIGMAPARTITION"
	SIGMANETFAIL        = "SIGMANETFAIL"
	SIGMAPERF           = "SIGMAPERF"
	SIGMADEBUG          = "SIGMADEBUG"
	SIGMANAMED          = "SIGMANAMED"
	SIGMAROOTFS         = "SIGMAROOTFS"
	SIGMAPATH           = "SIGMAPATH"
	SIGMAREALM          = "SIGMAREALM"
)

func GenPid() Tpid {
	return Tpid(rand.String(8))
}

func SetPid(pid Tpid) {
	os.Setenv(SIGMAPID, pid.String())
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

func SetNodedId(realmId string) {
	os.Setenv(SIGMANODEDID, realmId)
}

func GetNodedId() string {
	return os.Getenv(SIGMANODEDID)
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

func GetIsPrivilegedProc() bool {
	return os.Getenv(SIGMAPRIVILEGEDPROC) == "true"
}

func GetSigmaPerf() string {
	return os.Getenv(SIGMAPERF)
}

func GetSigmaDebug() string {
	return os.Getenv(SIGMADEBUG)
}

func GetSigmaNamed() string {
	return os.Getenv(SIGMANAMED)
}

func GetSigmaRootFs() string {
	return os.Getenv(SIGMAROOTFS)
}

func SetRealm(realm string) {
	os.Setenv(SIGMAREALM, realm)
}

func GetRealm() string {
	return os.Getenv(SIGMAREALM)
}

func GetSigmaPath() string {
	return os.Getenv(SIGMAPATH)
}

func GetLabels(envvar string) map[string]bool {
	m := make(map[string]bool)
	s := os.Getenv(envvar)
	if s == "" {
		return m
	}
	labels := strings.Split(s, ";")
	for _, l := range labels {
		m[l] = true
	}
	return m
}

func FakeProcEnv(pid Tpid, program, procdIp, procdir, parentdir string) {
	SetPid(pid)
	SetProgram(program)
	SetProcdIp(procdIp)
	SetProcDir(procdir)
	SetParentDir(parentdir)
}
