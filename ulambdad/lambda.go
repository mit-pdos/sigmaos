package ulambd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

type Attr struct {
	Program    string
	Args       []string
	AfterStart []string
	AfterExit  []string
}

type Lambda struct {
	ld         *Lambd
	pid        string
	path       string
	status     string
	program    string
	args       []string
	afterStart map[string]bool
	afterExit  map[string]bool
}

func (l *Lambda) String() string {
	str := fmt.Sprintf("λ pid %v st %v args %v start %v exit %v", l.pid, l.status,
		l.args, l.afterStart, l.afterExit)
	return str
}

func (l *Lambda) changeStatus(new string) error {
	err := l.ld.clnt.Rename(l.path+"/"+l.status, l.path+"/"+new)
	if err != nil {
		return fmt.Errorf("changeStatus %v to %v error %v\n",
			l.path+"/"+l.status, l.path+"/"+new, err)
	}
	l.status = new
	return nil
}

// XXX if remote, keep-alive?
func (l *Lambda) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("Lambda %v finished with error: %v", l, err)
	}
	l.ld.exit(l)
}

// XXX if had remote machines, this would be run on the remote machine
// maybe we should have machines register with ulambd; have a
// directory with machines?
func (l *Lambda) run() error {
	log.Printf("Run %v\n", l)
	err := l.changeStatus("Running")
	if err != nil {
		return err
	}
	args := append([]string{l.path}, l.args...)
	cmd := exec.Command(l.program, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}
	go l.wait(cmd)
	return nil
}

func (l *Lambda) runnable() bool {
	return len(l.afterExit) == 0 && len(l.afterStart) == 0
}
