package realm

import (
	"log"
	"os/exec"
	"strings"
	"syscall"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
)

const (
	TEST_RID = "1000"
)

type TestEnv struct {
	bin       string
	rid       string
	namedPids []string
	namedCmds []*exec.Cmd
	realmmgr  *exec.Cmd
	machined  []*exec.Cmd
	clnt      *RealmClnt
}

func MakeTestEnv(bin string) *TestEnv {
	e := &TestEnv{}
	e.bin = bin
	e.rid = TEST_RID
	e.namedPids = []string{}
	e.namedCmds = []*exec.Cmd{}
	e.machined = []*exec.Cmd{}

	return e
}

func (e *TestEnv) Boot() (*RealmConfig, error) {
	if err := e.bootNameds(); err != nil {
		log.Printf("nameds")
		return nil, err
	}
	clnt := MakeRealmClnt()
	e.clnt = clnt
	if err := e.bootRealmMgr(); err != nil {
		log.Printf("realmmgr")
		return nil, err
	}
	if err := e.BootMachined(); err != nil {
		log.Printf("machined")
		return nil, err
	}
	cfg := clnt.CreateRealm(e.rid)
	return cfg, nil
}

// TODO: eventually wait on exit signals
func (e *TestEnv) Shutdown() {
	log.Printf("Start shutdown")
	// Destroy the realm
	e.clnt.DestroyRealm(e.rid)
	log.Printf("Destroyed realm")

	// Kill the machined
	for _, machined := range e.machined {
		kill(machined)
	}
	log.Printf("killed machineds")
	e.machined = []*exec.Cmd{}

	// Kill the realmmgr
	kill(e.realmmgr)
	log.Printf("killed realmmgr")
	e.realmmgr = nil

	for _, addr := range fslib.Named() {
		ShutdownNamed(addr)
	}

	//ShutdownNamedReplicas(fslib.Named())
}

func (e *TestEnv) bootNameds() error {
	namedCmds, err := BootNamedReplicas(e.bin, fslib.Named(), kernel.NO_REALM)
	e.namedCmds = namedCmds
	// Start a named instance.
	if err != nil {
		log.Fatalf("Error BootNamedReplicas in TestEnv.BootNameds: %v", err)
		return err
	}
	return nil
}

func (e *TestEnv) bootRealmMgr() error {
	p := proc.MakeProcPid("realmmgr-"+proc.GenPid(), "bin/realm/realmmgr", []string{e.bin})
	cmd, err := e.clnt.SpawnKernelProc(p, e.bin, fslib.Named())
	if err != nil {
		return err
	}
	e.realmmgr = cmd
	return e.clnt.WaitStart(p.Pid)
}

func (e *TestEnv) BootMachined() error {
	var err error
	machined, err := proc.Run("0", e.bin, "bin/realm/machined", fslib.Named(), []string{e.bin, proc.GenPid()})
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
