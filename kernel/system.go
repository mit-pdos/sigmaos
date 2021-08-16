package kernel

import (
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"ulambda/fslib"
)

const (
	NAMED = "name"
	PROCD = "name/procd"
	S3    = "name/s3"
	UX    = "name/ux"
)

const (
	POST_BOOT_SLEEP_MS = 1000
)

type System struct {
	named *exec.Cmd
	nps3d *exec.Cmd
	npuxd *exec.Cmd
	procd *exec.Cmd
}

func run(bin string, name string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	return cmd, cmd.Start()
}

func BootMin(bin string) (*System, error) {
	s := &System{}
	cmd, err := run(bin, "/bin/memfsd", []string{"0", ":1111"})
	if err != nil {
		return nil, err
	}
	s.named = cmd
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
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
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	err = s.BootNps3d(bin)
	if err != nil {
		return nil, err
	}
	s.procd, err = run(bin, "/bin/procd", []string{bin})
	if err != nil {
		return nil, err
	}
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	return s, nil
}

func (s *System) BootNpUxd(bin string) error {
	var err error
	s.npuxd, err = run(bin, "/bin/npuxd", nil)
	if err != nil {
		return err
	}
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	return nil
}

func (s *System) BootNps3d(bin string) error {
	var err error
	s.nps3d, err = run(bin, "/bin/nps3d", nil)
	if err != nil {
		return err
	}
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	return nil
}

func (s *System) BootProcd(bin string) error {
	var err error
	s.procd, err = run(bin, "/bin/procd", []string{bin})
	if err != nil {
		return err
	}
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	return nil
}

func (s *System) RmUnionDir(clnt *fslib.FsLib, mdir string) error {
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

func (s *System) Kill(srv string) error {
	var err error
	switch srv {
	case PROCD:
		if s.procd != nil {
			err = s.procd.Process.Kill()
			if err == nil {
				s.procd = nil
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
	default:
	}
	return nil
}

func (s *System) Shutdown(clnt *fslib.FsLib) {
	if s.nps3d != nil {
		err := s.RmUnionDir(clnt, S3)
		if err != nil {
			log.Printf("S3 shutdown %v\n", err)
		}
		s.nps3d.Wait()

	}
	if s.npuxd != nil {
		err := s.RmUnionDir(clnt, UX)
		if err != nil {
			log.Printf("Ux shutdown %v\n", err)
		}
		s.npuxd.Wait()
	}
	if s.procd != nil {
		err := s.RmUnionDir(clnt, PROCD)
		if err != nil {
			log.Printf("Procds shutdown %v\n", err)
		}
		s.procd.Wait()
	}

	// Shutdown named last
	err := clnt.Remove(NAMED + "/")
	if err != nil {
		// XXX sometimes we get EOF....
		if err.Error() == "EOF" {
			log.Printf("Remove %v shutdown %v\n", NAMED, err)
		} else {
			log.Fatalf("Remove %v shutdown %v\n", NAMED, err)
		}
	}
	s.named.Wait()
}
