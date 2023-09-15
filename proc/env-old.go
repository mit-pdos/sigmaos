package proc

import (
	"os"
	"strings"
)

// Environment variables which every proc expects to have.
const (
	SIGMADEBUGPID = "SIGMADEBUGPID"
	SIGMAPERF     = "SIGMAPERF"
	SIGMADEBUG    = "SIGMADEBUG"
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
