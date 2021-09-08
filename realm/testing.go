package realm

import (
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
)

const (
	TEST_RID = "1000"
)

type TestEnv struct {
	bin      string
	rid      string
	realmmgr *exec.Cmd
	realmd   []*exec.Cmd
	clnt     *RealmClnt
}

func MakeTestEnv(bin string) *TestEnv {
	e := &TestEnv{}
	e.bin = bin
	e.rid = TEST_RID
	e.realmd = []*exec.Cmd{}

	return e
}

func (e *TestEnv) Boot() (*RealmConfig, error) {
	if err := e.bootRealmMgr(); err != nil {
		return nil, err
	}
	if err := e.BootRealmd(); err != nil {
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

	// Kill the realmd
	for _, realmd := range e.realmd {
		kill(realmd)
	}
	e.realmd = []*exec.Cmd{}

	// Kill the realmmgr
	kill(e.realmmgr)
	e.realmmgr = nil

	ShutdownNamed(fslib.Named())
}

func (e *TestEnv) bootRealmMgr() error {
	// Create boot cond
	var err error
	realmmgr, err := run(e.bin, "bin/realm/realmmgr", fslib.Named(), []string{e.bin})
	e.realmmgr = realmmgr
	if err != nil {
		return err
	}
	time.Sleep(SLEEP_MS * 2 * time.Millisecond)
	return nil
}

func (e *TestEnv) BootRealmd() error {
	var err error
	realmd, err := run(e.bin, "bin/realm/realmd", fslib.Named(), []string{e.bin})
	e.realmd = append(e.realmd, realmd)
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
		log.Fatalf("Error realmd Wait in kill: %v", err)
	}
}
