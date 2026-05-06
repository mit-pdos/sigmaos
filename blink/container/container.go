package blink

import (
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/linux/mem"
	"sigmaos/util/perf"
)

const (
	JUNCTION_RUN    = "/home/arielck/blink-ae/junction/build/junction/junction_run"
	JUNCTION_CONFIG = "/home/arielck/blink-ae/junction.config" // TODO: confirm config path
	JUNCTION_CHROOT = "/home/arielck/blink-ae/chroot"
	SNAPSHOT_JM     = "/tmp/python_helloworld.jm"                     // TODO: derive from proc
	SNAPSHOT_JIF    = "/tmp/python_helloworld_itrees_ord_reorder.jif" // TODO: derive from proc
)

type BlinkContainer struct {
	cmd *exec.Cmd
	pid int // PID of the restored process; TODO: replace with PID communicated by junction_run
}

// StartBlinkContainer restores a proc from a Blink snapshot by invoking
// junction_run. Blink manages snapshot storage and binary staging
// transparently.
func StartBlinkContainer(uproc *proc.Proc) (*BlinkContainer, error) {
	functionArg := ""
	if args := uproc.GetArgs(); len(args) > 0 {
		functionArg = args[0]
	}

	cmd := exec.Command("sudo", "-E",
		JUNCTION_RUN,
		JUNCTION_CONFIG,
		"--chroot="+JUNCTION_CHROOT,
		"--cache_linux_fs",
		"--function_arg", functionArg,
		"--function_name", uproc.GetProgram(),
		"--jif",
		"-rk",
		"--",
		SNAPSHOT_JM,
		SNAPSHOT_JIF,
	)

	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		db.DFatalf("Error marshal timestamp: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	cmd.Env = uproc.GetEnv()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	db.DPrintf(db.CONTAINER, "StartBlinkContainer %v function_name %v", JUNCTION_RUN, uproc.GetProgram())

	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "StartBlinkContainer err %v: %v", cmd, err)
		return nil, err
	}
	perf.LogSpawnLatency("StartBlinkContainer cmd.Start", uproc.GetPid(), uproc.GetSpawnTime(), s)

	// TODO: replace with PID communicated by junction_run
	return &BlinkContainer{cmd: cmd, pid: rand.Int()}, nil
}

func (bc *BlinkContainer) Pid() int {
	return bc.pid
}

func (bc *BlinkContainer) GetPSS() (proc.Tmem, error) {
	return mem.GetAggregatePSS(bc.pid)
}

func (bc *BlinkContainer) Wait() error {
	return bc.cmd.Wait()
}

// CleanupBlinkProc is a placeholder; Blink manages its own cleanup.
func CleanupBlinkProc(_ sp.Tpid) {}
