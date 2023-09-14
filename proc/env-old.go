package proc

import (
	"os"
	"strings"

	sp "sigmaos/sigmap"
)

// Environment variables which every proc expects to have.
const (
	SIGMADEBUGPID  = "SIGMADEBUGPID"
	SIGMACRASH     = "SIGMACRASH"
	SIGMAPARTITION = "SIGMAPARTITION"
	SIGMANETFAIL   = "SIGMANETFAIL"
	SIGMAPERF      = "SIGMAPERF"
	SIGMADEBUG     = "SIGMADEBUG"
	SIGMANET       = "SIGMANET"
	SIGMAJAEGERIP  = "SIGMAJAEGERIP"
	SIGMAUPROCD    = "SIGMAUPROCD"
)

func SetSigmaDebugPid(pid string) {
	os.Setenv(SIGMADEBUGPID, pid)
}

func GetSigmaDebugPid() string {
	return os.Getenv(SIGMADEBUGPID)
}

func GetSigmaPerf() string {
	return os.Getenv(SIGMAPERF)
}

func GetSigmaDebug() string {
	return os.Getenv(SIGMADEBUG)
}

func SetSigmaJaegerIP(ip string) {
	os.Setenv(SIGMAJAEGERIP, ip)
}

func GetSigmaJaegerIP() string {
	return os.Getenv(SIGMAJAEGERIP)
}

func GetNet() string {
	return os.Getenv(SIGMANET)
}

func GetUprocdPid() sp.Tpid {
	return sp.Tpid(os.Getenv(SIGMAUPROCD))
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
