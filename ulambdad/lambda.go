package ulambd

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"ulambda/fslib"
)

type Attr struct {
	Program    string
	Args       []string
	AfterStart []string
	AfterExit  []string
}

type Lambda struct {
	clnt   *fslib.FsLib
	pid    string
	status string
	attr   Attr
}

func (l *Lambda) String() string {
	str := fmt.Sprintf("λ pid %v st %v args %v start %v exit %v", l.pid, l.status,
		l.attr.Args, l.attr.AfterStart, l.attr.AfterExit)
	return str
}

func (l *Lambda) changeStatus(new string) error {
	err := l.clnt.Rename(l.pid+"/"+l.status, l.pid+"/"+new)
	if err != nil {
		return fmt.Errorf("changeStatus %v to %v error %v\n",
			l.pid+"/"+l.status, l.pid+"/"+new, err)
	}
	l.status = new
	return nil
}

// XXX if had remote machines, this would be run on the remote machine
// maybe we should have machines register with ulambd; have a
// directory with machines?
func (l *Lambda) run() error {
	err := l.changeStatus("Running")
	if err != nil {
		return err
	}
	args := append([]string{l.pid}, l.attr.Args...)
	cmd := exec.Command(l.attr.Program, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func (l *Lambda) exit() error {
	err := l.clnt.Remove(l.pid)
	if err != nil {
		return fmt.Errorf("Remove %v error %v\n", l.pid, err)
	}
	return nil
}

func (l *Lambda) isRunnable(pids map[string]bool) bool {
	log.Printf("isRunnable start %v exit %v pids %v\n", l.attr.AfterStart,
		l.attr.AfterExit, pids)

	for _, pid := range l.attr.AfterStart {
		_, ok := pids[pid]
		if !ok {
			return false
		}
	}

	// all start dependencies have started

	for _, pid := range l.attr.AfterExit {
		_, ok := pids[pid]
		if ok {
			return false
		}
	}
	err := l.changeStatus("Runnable")
	if err != nil {
		return false
	}
	return true
}
