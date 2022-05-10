package realm

import (
	"log"
	"os/exec"
	"strings"
	"syscall"

	db "ulambda/debug"
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
	sigmamgr  *exec.Cmd
	noded     []*exec.Cmd
	*RealmClnt
}

func MakeTestEnv(bin string) *TestEnv {
	e := &TestEnv{}
	e.bin = bin
	e.rid = TEST_RID
	e.namedPids = []string{}
	e.namedCmds = []*exec.Cmd{}
	e.noded = []*exec.Cmd{}

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
	if err := e.BootNoded(); err != nil {
		log.Printf("noded")
		return nil, err
	}
	cfg := e.CreateRealm(e.rid)
	return cfg, nil
}

func (e *TestEnv) Shutdown() {
	// Destroy the realm
	e.DestroyRealm(e.rid)

	// Kill the noded
	for _, noded := range e.noded {
		kill(noded)
	}
	e.noded = []*exec.Cmd{}

	// Kill the sigmamgr
	kill(e.sigmamgr)
	e.sigmamgr = nil

	for _, namedCmd := range e.namedCmds {
		kill(namedCmd)
	}
}

func (e *TestEnv) bootNameds() error {
	namedCmds, err := BootNamedReplicas(e.bin, fslib.Named(), kernel.NO_REALM)
	e.namedCmds = namedCmds
	// Start a named instance.
	if err != nil {
		db.DFatalf("Error BootNamedReplicas in TestEnv.BootNameds: %v", err)
		return err
	}
	return nil
}

func (e *TestEnv) bootSigmaMgr() error {
	p := proc.MakeProcPid("sigmamgr-"+proc.GenPid(), "bin/realm/sigmamgr", []string{e.bin})
	cmd, err := e.RealmClnt.SpawnKernelProc(p, e.bin, fslib.Named())
	if err != nil {
		return err
	}
	e.sigmamgr = cmd
	return e.RealmClnt.WaitStart(p.Pid)
}

func (e *TestEnv) BootNoded() error {
	var err error
	p := proc.MakeProcPid(proc.Tpid("0"), "/bin/realm/noded", []string{e.bin, proc.GenPid().String()})
	noded, err := proc.RunKernelProc(p, e.bin, fslib.Named())
	e.noded = append(e.noded, noded)
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
		db.DFatalf("Error noded Wait in kill: %v", err)
	}
}
