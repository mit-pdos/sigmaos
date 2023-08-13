package proc

import (
	"os"
	"strings"

	sp "sigmaos/sigmap"
)

// Environment variables which every proc expects to have.
const (
	SIGMAPRIVILEGEDPROC = "SIGMAKERNELPROC"
	SIGMAPID            = "SIGMAPID"
	SIGMAPROGRAM        = "SIGMAPROGRAM"
	SIGMAPROCDIR        = "SIGMAPROCDIR"
	SIGMAPARENTDIR      = "SIGMAPARENTDIR"
	SIGMACRASH          = "SIGMACRASH"
	SIGMAPARTITION      = "SIGMAPARTITION"
	SIGMANETFAIL        = "SIGMANETFAIL"
	SIGMAPERF           = "SIGMAPERF"
	SIGMADEBUG          = "SIGMADEBUG"
	//	SIGMANAMED          = "SIGMANAMED"
	SIGMALOCAL    = "SIGMALOCAL"
	SIGMATAG      = "SIGMATAG"
	SIGMAROOTFS   = "SIGMAROOTFS"
	SIGMAREALM    = "SIGMAREALM"
	SIGMANET      = "SIGMANET"
	SIGMAJAEGERIP = "SIGMAJAEGERIP"
	SIGMAKERNEL   = "SIGMAKERNEL"
	SIGMAUPROCD   = "SIGMAUPROCD"
)

func SetPid(pid sp.Tpid) {
	os.Setenv(SIGMAPID, pid.String())
}

// Can return "" for test programs that make a procclnt
func GetPid() sp.Tpid {
	return sp.Tpid(os.Getenv(SIGMAPID))
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

//func NamedAddrs() string {
//	addrs := GetSigmaNamed()
//	if addrs == "" {
//		return "127.0.0.1"
//		debug.PrintStack()
//		log.Fatalf("Getenv error: missing SIGMANAMED")
//	}
//	return addrs
//}
//
//func Named() (sp.Taddrs, error) {
//	return sp.String2Taddrs(NamedAddrs())
//}
//
//func SetSigmaNamed(nds sp.Taddrs) error {
//	return nil
//	s, err := nds.Taddrs2String()
//	if err != nil {
//		return err
//	}
//	os.Setenv(SIGMANAMED, s)
//	return nil
//}
//
//func GetSigmaNamed() string {
//	return os.Getenv(SIGMANAMED)
//}

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

func GetSigmaLocal() string {
	return os.Getenv(SIGMALOCAL)
}

func SetSigmaLocal(ip string) {
	os.Setenv(SIGMALOCAL, ip)
}

func GetSigmaRootFs() string {
	return os.Getenv(SIGMAROOTFS)
}

func SetSigmaJaegerIP(ip string) {
	os.Setenv(SIGMAJAEGERIP, ip)
}

func GetSigmaJaegerIP() string {
	return os.Getenv(SIGMAJAEGERIP)
}

func GetRealm() sp.Trealm {
	return sp.Trealm(os.Getenv(SIGMAREALM))
}

func GetKernelId() string {
	return os.Getenv(SIGMAKERNEL)
}

func GetNet() string {
	return os.Getenv(SIGMANET)
}

func GetUprocdPid() sp.Tpid {
	return sp.Tpid(os.Getenv(SIGMAUPROCD))
}

func GetBuildTag() string {
	return os.Getenv(SIGMATAG)
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

func FakeProcEnv(pid sp.Tpid, program, procdir, parentdir string) {
	SetPid(pid)
	SetProgram(program)
	SetProcDir(procdir)
	SetParentDir(parentdir)
}
