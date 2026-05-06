package proc

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	sp "sigmaos/sigmap"
)

func TestWriteProcEnv(t *testing.T) {
	uproc := NewProcPid(sp.Tpid("imgrec-blink-1"), "imgrec-blink.py", []string{})

	// FinalizeEnv as done in sched/msched/proc/srv/srv.go
	innerIP := sp.Tip("127.0.0.1")
	outerIP := sp.Tip("10.0.0.1")
	procdPid := sp.GenPid("procd")
	uproc.FinalizeEnv(innerIP, outerIP, procdPid)

	// Append env vars as done in scontainer/scontainer.go
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		t.Fatalf("Error marshal timestamp: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(SIGMAPERF, uproc.GetProcEnv().GetPerf())

	env := uproc.GetEnv()
	if err := os.WriteFile("/tmp/proc-env.txt", []byte(strings.Join(env, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("Error writing env file: %v", err)
	}
}
