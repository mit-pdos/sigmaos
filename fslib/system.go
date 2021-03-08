package fslib

import (
	"log"

	"os"
	"os/exec"
	"time"
)

const NAMED = "name"
const S3 = "name/s3"
const UX = "name/ux"

type System struct {
	named  *exec.Cmd
	schedd *exec.Cmd
	nps3d  *exec.Cmd
}

func run(name string) (*exec.Cmd, error) {
	cmd := exec.Command(name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
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
	s.nps3d, err = run(bin + "/nps3d")
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

	// Remove pool of s3 servers
	dirents, err := clnt.ReadDir("name/s3")
	if err != nil {
		log.Fatalf("Nps3d shutdown %v\n", err)
	}
	for _, st := range dirents {
		err = clnt.Remove(S3 + "/" + st.Name + "/")
		if err != nil {
			log.Fatalf("S3 shutdown %v\n", err)
		}
		err = clnt.Remove(S3 + "/" + st.Name)
		if err != nil {
			log.Fatalf("S3 shutdown %v\n", err)
		}
	}

	// Shutdown named last
	err = clnt.Remove(NAMED + "/")
	if err != nil {
		log.Fatalf("Named shutdown %v\n", err)
	}

	s.schedd.Wait()
	s.named.Wait()
	s.nps3d.Wait()
}
