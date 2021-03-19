package locald

import (
	"encoding/json"
	//	"github.com/sasha-s/go-deadlock"
	"log"
	"os"
	"os/exec"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
)

type Lambda struct {
	//	mu deadlock.Mutex
	mu      sync.Mutex
	Program string
	Pid     string
	Args    []string
	Env     []string
	Dir     string
	SysPid  int
	attr    *fslib.Attr
	ld      *LocalD
	// XXX add fields (e.g. CPU mask, etc.)
}

// XXX init/run pattern seems a bit redundant...
func (l *Lambda) init(a []byte) error {
	var attr fslib.Attr
	err := json.Unmarshal(a, &attr)
	if err != nil {
		log.Printf("Locald unmarshalling error\n: %v", err)
		return err
	}
	l.Program = attr.Program
	l.Pid = attr.Pid
	l.Args = attr.Args
	l.Env = attr.Env
	l.Dir = attr.Dir
	l.attr = &attr
	db.DLPrintf(l.ld.name, "LOCALD", "Locald init: %v\n", attr)
	return nil
}

func (l *Lambda) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("Lambda %v finished with error: %v", l.attr, err)
		// XXX Need to think about how to return errors
		//		return err
	}

	// XXX Race condition in fslib requires this to be locked
	l.ld.mu.Lock()
	defer l.ld.mu.Unlock()

	// Notify schedd that the process exited
	l.ld.Exiting(l.attr.Pid, "OK")
}

func (l *Lambda) run() error {
	db.DLPrintf(l.ld.name, "LOCALD", "Locald run: %v\n", l.attr)
	args := append([]string{l.Pid}, l.Args...)
	env := append(os.Environ(), l.Env...)
	cmd := exec.Command(l.ld.bin+"/"+l.Program, args...)
	cmd.Env = env
	cmd.Dir = l.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Printf("Locald run error: %v, %v\n", l.attr, err)
		return err
	}

	l.SysPid = cmd.Process.Pid

	go l.wait(cmd)
	return nil
}
