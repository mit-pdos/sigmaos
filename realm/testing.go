package realm

import (
	"log"
	"os/exec"
	"strings"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernel"
	"sigmaos/proc"
)

type TestEnv struct {
	rid       string
	namedPids []string
	namedCmds []*exec.Cmd
	sigmamgr  *exec.Cmd
	machined  []*exec.Cmd
	*RealmClnt
}

func MakeTestEnv(rid string) *TestEnv {
	e := &TestEnv{}
	e.rid = rid
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
	e.RealmClnt = clnt
	if err := e.bootSigmaMgr(); err != nil {
		log.Printf("sigmamgr")
		return nil, err
	}
	if err := e.BootMachined(); err != nil {
		log.Printf("machined")
		return nil, err
	}
	cfg := e.CreateRealm(e.rid)
	return cfg, nil
}

func (e *TestEnv) Shutdown() {
	db.DPrintf("TEST", "Shutting down")
	// Destroy the realm
	e.DestroyRealm(e.rid)

	// Kill the machined
	for _, machined := range e.machined {
		kill(machined)
	}
	e.machined = []*exec.Cmd{}

	// Kill the sigmamgr
	kill(e.sigmamgr)
	e.sigmamgr = nil

	for _, namedCmd := range e.namedCmds {
		kill(namedCmd)
	}
}

func (e *TestEnv) bootNameds() error {
	namedCmds, err := BootNamedReplicas(fslib.Named(), kernel.NO_REALM)
	e.namedCmds = namedCmds
	// Start a named instance.
	if err != nil {
		db.DFatalf("Error BootNamedReplicas in TestEnv.BootNameds: %v", err)
		return err
	}
	return nil
}

func (e *TestEnv) bootSigmaMgr() error {
	p := proc.MakeProcPid("sigmamgr-"+proc.GenPid(), "realm/sigmamgr", []string{})
	cmd, err := e.RealmClnt.SpawnKernelProc(p, fslib.Named())
	if err != nil {
		return err
	}
	e.sigmamgr = cmd
	return e.RealmClnt.WaitStart(p.Pid)
}

func (e *TestEnv) BootMachined() error {
	var err error
	pid := proc.Tpid("machined-" + proc.GenPid().String())
	p := proc.MakeProcPid(pid, "realm/machined", []string{})
	machined, err := proc.RunKernelProc(p, fslib.Named())
	e.machined = append(e.machined, machined)
	if err != nil {
		return err
	}
	return nil
}

func kill(cmd *exec.Cmd) {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		db.DFatalf("Error Kill in kill: %v", err)
	}
	if err := cmd.Wait(); err != nil && !strings.Contains(err.Error(), "signal") {
		db.DFatalf("Error machined Wait in kill: %v", err)
	}
}
