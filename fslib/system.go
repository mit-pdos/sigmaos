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
	npuxd  *exec.Cmd
	locald *exec.Cmd
}

func run(bin string, name string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(bin+"/"+name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	return cmd, cmd.Start()
}

func BootMin(bin string) (*System, error) {
	s := &System{}
	cmd, err := run(bin, "/bin/named", nil)
	if err != nil {
		return nil, err
	}
	s.named = cmd
	time.Sleep(100 * time.Millisecond)
	return s, nil
}

func Boot(bin string) (*System, error) {
	s, err := BootMin(bin)
	if err != nil {
		return nil, err
	}
	s.npuxd, err = run(bin, "/bin/npuxd", nil)
	if err != nil {
		return nil, err
	}
	time.Sleep(100 * time.Millisecond)
	s.nps3d, err = run(bin, "/bin/nps3d", nil)
	if err != nil {
		return nil, err
	}
	time.Sleep(100 * time.Millisecond)
	s.schedd, err = run(bin, "/bin/schedd", nil)
	if err != nil {
		return nil, err
	}
	time.Sleep(100 * time.Millisecond)

	s.locald, err = run(bin, "/bin/locald", []string{bin})
	if err != nil {
		return nil, err
	}
	time.Sleep(100 * time.Millisecond)
	return s, nil
}

func (s *System) RmUnionDir(clnt *FsLib, mdir string) error {
	dirents, err := clnt.ReadDir(mdir)
	if err != nil {
		return err
	}
	for _, st := range dirents {
		err = clnt.Remove(mdir + "/" + st.Name + "/")
		if err != nil {
			return err
		}
		err = clnt.Remove(mdir + "/" + st.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *System) Shutdown(clnt *FsLib) {
	if s.schedd != nil {
		err := clnt.Remove(SCHED + "/")
		if err != nil {
			log.Fatalf("Schedd shutdown %v\n", err)
		}
		s.schedd.Wait()
	}
	if s.nps3d != nil {
		err := s.RmUnionDir(clnt, "name/s3")
		if err != nil {
			log.Fatalf("S3 shutdown %v\n", err)
		}
		s.nps3d.Wait()

	}
	if s.npuxd != nil {
		err := s.RmUnionDir(clnt, "name/ux")
		if err != nil {
			log.Fatalf("Ux shutdown %v\n", err)
		}
		s.npuxd.Wait()
	}

	// Shutdown named last
	err := clnt.Remove(NAMED + "/")
	if err != nil {
		log.Fatalf("Named shutdown %v\n", err)
	}
	s.named.Wait()
}
