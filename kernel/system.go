package kernel

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"ulambda/fslib"
	"ulambda/sync"
)

const (
	NAMED = "name"
	PROCD = "name/procd"
	S3    = "name/s3"
	UX    = "name/ux"
	BOOT  = "name/boot"
)

const (
	POST_BOOT_SLEEP_MS = 1000
)

type System struct {
	named *exec.Cmd
	nps3d []*exec.Cmd
	npuxd []*exec.Cmd
	procd []*exec.Cmd
	*fslib.FsLib
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
	cmd, err := run(bin, "/bin/kernel/memfsd", []string{"0", ":1111"})
	if err != nil {
		return nil, err
	}
	s.named = cmd
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	s.FsLib = fslib.MakeFsLib("kernel")
	err = s.Mkdir(BOOT, 0777)
	if err != nil {
		log.Printf("Error creating boot dir: %v", err)
		return s, err
	}
	err = s.Mkdir(fslib.LOCKS, 0777)
	if err != nil {
		log.Printf("Error creating boot dir: %v", err)
		return s, err
	}
	return s, nil
}

func Boot(bin string) (*System, error) {
	s, err := BootMin(bin)
	if err != nil {
		return nil, err
	}
	err = s.BootNpUxd(bin)
	if err != nil {
		return nil, err
	}
	err = s.BootNps3d(bin)
	if err != nil {
		return nil, err
	}
	err = s.BootProcd(bin)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *System) BootNpUxd(bin string) error {
	// Create boot cond
	pid := "npuxd-" + fslib.GenPid()
	npuxdStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	npuxdStartCond.Init()
	var err error
	npuxd, err := run(bin, "bin/kernel/npuxd", []string{pid})
	s.npuxd = append(s.npuxd, npuxd)
	if err != nil {
		return err
	}
	// Wait for boot
	npuxdStartCond.Wait()
	return nil
}

func (s *System) BootNps3d(bin string) error {
	// Create boot cond
	pid := "nps3d-" + fslib.GenPid()
	nps3dStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	nps3dStartCond.Init()
	var err error
	nps3d, err := run(bin, "bin/kernel/nps3d", []string{pid})
	s.nps3d = append(s.nps3d, nps3d)
	if err != nil {
		return err
	}
	// Wait for boot
	nps3dStartCond.Wait()
	return nil
}

func (s *System) BootProcd(bin string) error {
	// Create boot cond
	pid := "procd-" + fslib.GenPid()
	procdStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	procdStartCond.Init()
	var err error
	procd, err := run(bin, "bin/kernel/procd", []string{bin, pid})
	s.procd = append(s.procd, procd)
	if err != nil {
		return err
	}
	// Wait for boot
	procdStartCond.Wait()
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
		if err != nil && !strings.Contains(err.Error(), "file not found") {
			return err
		}
	}
	return nil
}

func (s *System) KillOne(srv string) error {
	var err error
	switch srv {
	case PROCD:
		if len(s.procd) > 0 {
			log.Printf("Killing a procd")
			err = s.procd[0].Process.Kill()
			if err == nil {
				s.procd[0].Wait()
				s.procd = s.procd[1:]
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
	default:
	}
	return nil
}

func (s *System) Shutdown(clnt *fslib.FsLib) {
	if len(s.nps3d) != 0 {
		err := s.RmUnionDir(clnt, S3)
		if err != nil {
			log.Printf("S3 shutdown %v\n", err)
		}
		for _, d := range s.nps3d {
			d.Wait()
		}
	}
	if len(s.npuxd) != 0 {
		err := s.RmUnionDir(clnt, UX)
		if err != nil {
			log.Printf("Ux shutdown %v\n", err)
		}
		for _, d := range s.npuxd {
			d.Wait()
		}
	}
	if len(s.procd) != 0 {
		err := s.RmUnionDir(clnt, PROCD)
		if err != nil {
			log.Printf("Procds shutdown %v\n", err)
		}
		for _, d := range s.procd {
			d.Wait()
		}
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
