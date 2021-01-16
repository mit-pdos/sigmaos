package ulambd

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"ulambda/fslib"
)

type Attr struct {
	Program      string
	Args         []string
	Dependencies []string
}

type Lambda struct {
	clnt   *fslib.FsLib
	pid    string
	status string
	attr   Attr
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

func (l *Lambda) Run() error {
	err := l.changeStatus("Running")
	if err != nil {
		return err
	}
	args := append([]string{l.pid}, l.attr.Args...)
	cmd := exec.Command(l.attr.Program, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}
	err = l.changeStatus("Done")
	if err != nil {
		return err
	}
	err = l.clnt.Remove(l.pid)
	if err != nil {
		return fmt.Errorf("Remove %v error %v\n", l.pid, err)
	}
	return nil
}

// XXX move stuff from run into exit

func (l *Lambda) isRunnable(pids map[string]bool) bool {
	log.Printf("isRunnable deps %v pids %v\n", l.attr.Dependencies, pids)
	for _, pid := range l.attr.Dependencies {
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
