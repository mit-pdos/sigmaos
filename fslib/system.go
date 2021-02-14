package fslib

import (
	"log"

	"os"
	"os/exec"
	"time"
)

const NAMEDEV = "name/dev"

type System struct {
	named  *exec.Cmd
	schedd *exec.Cmd
}

func run(name string) (*exec.Cmd, error) {
	cmd := exec.Command(name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, cmd.Start()
}

func Boot(bin string) (*System, error) {
	s := &System{}

	cmd, err := run(bin + "/named")
	if err != nil {
		return nil, err
	}
	s.named = cmd
	s.schedd, err = run(bin + "/schedd")
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	return s, nil
}

func (s *System) Shutdown(clnt *FsLib) {
	err := clnt.Remove(SCHED + "/")
	if err != nil {
		log.Fatalf("Schedd shutdown %v\n", err)
	}
	err = clnt.WriteFile(NAMEDEV, []byte("Exit"))
	if err != nil {
		log.Fatalf("Named shutdown %v\n", err)
	}
	s.schedd.Wait()
	s.named.Wait()
}
