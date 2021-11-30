package realm

import (
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
	"ulambda/procd"
)

const (
	TEST_RID = "1000"
)

type TestEnv struct {
	bin      string
	rid      string
	realmmgr *exec.Cmd
	machined []*exec.Cmd
	clnt     *RealmClnt
}

func MakeTestEnv(bin string) *TestEnv {
	e := &TestEnv{}
	e.bin = bin
	e.rid = TEST_RID
	e.machined = []*exec.Cmd{}

	return e
}

func (e *TestEnv) Boot() (*RealmConfig, error) {
	if err := e.bootRealmMgr(); err != nil {
		return nil, err
	}
	if err := e.BootMachined(); err != nil {
		return nil, err
	}
	clnt := MakeRealmClnt()
	e.clnt = clnt
	cfg := clnt.CreateRealm(e.rid)
	return cfg, nil
}

func (e *TestEnv) Shutdown() {
	// Destroy the realm
	e.clnt.DestroyRealm(e.rid)

	// Kill the machined
	for _, machined := range e.machined {
		kill(machined)
	}
	e.machined = []*exec.Cmd{}

	// Kill the realmmgr
	kill(e.realmmgr)
	e.realmmgr = nil

	ShutdownNamedReplicas(fslib.Named())
}

func (e *TestEnv) bootRealmMgr() error {
	// Create boot cond
	var err error
	realmmgr, err := procd.Run("0", e.bin, "bin/realm/realmmgr", fslib.Named(), []string{e.bin})
	e.realmmgr = realmmgr
	if err != nil {
		return err
	}
	time.Sleep(kernel.SLEEP_MS * time.Millisecond)
	fsl := fslib.MakeFsLib("testenv")
	WaitRealmMgrStart(fsl)
	return nil
}

func (e *TestEnv) BootMachined() error {
	var err error
	machined, err := procd.Run("0", e.bin, "bin/realm/machined", fslib.Named(), []string{e.bin, proc.GenPid()})
	e.machined = append(e.machined, machined)
	if err != nil {
		return err
	}
	return nil
}

func kill(cmd *exec.Cmd) {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		log.Fatalf("Error Kill in kill: %v", err)
	}
	if err := cmd.Wait(); err != nil && !strings.Contains(err.Error(), "signal") {
		log.Fatalf("Error machined Wait in kill: %v", err)
	}
}
