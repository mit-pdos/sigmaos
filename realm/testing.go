package realm

import (
	"log"
	"os/exec"
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
	e.realmd = []*exec.Cmd{}
	cfg := clnt.CreateRealm(e.rid)
	time.Sleep(500 * time.Millisecond)
	return cfg, nil
}

func (e *TestEnv) Shutdown() {
	// Destroy the realm
	e.clnt.DestroyRealm(e.rid)

	time.Sleep(SLEEP_MS * time.Millisecond)

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
	// Create boot cond
	var err error
	realmd, err := run(e.bin, "bin/realm/realmd", fslib.Named(), []string{e.bin})
	e.realmd = append(e.realmd, realmd)
	if err != nil {
		return err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)
	return nil
}

func kill(cmd *exec.Cmd) {
	if err := cmd.Process.Kill(); err != nil {
		log.Fatalf("Error Kill in kill: %v", err)
	}
	if err := cmd.Wait(); err != nil && err.Error() != "signal: killed" {
		log.Fatalf("Error realmd Wait in kill: %v", err)
	}
}
