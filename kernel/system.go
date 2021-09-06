package kernel

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
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
	bin       string
	namedAddr string
	named     *exec.Cmd
	fss3d     []*exec.Cmd
	fsuxd     []*exec.Cmd
	procd     []*exec.Cmd
	*fslib.FsLib
}

// XXX To be removed
func MakeSystem(bin string) *System {
	return MakeSystemNamedAddr(bin, fslib.Named())
}

func MakeSystemNamedAddr(bin string, namedAddr string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = namedAddr
	return s
}

// XXX To be removed
func (s *System) BootFollower() error {
	err := s.BootFsUxd()
	if err != nil {
		return err
	}
	err = s.BootFss3d()
	if err != nil {
		return err
	}
	err = s.BootProcd()
	if err != nil {
		return err
	}
	return nil
}

// XXX To be removed
func (s *System) BootMin() error {
	return s.BootNamed(fslib.Named())
}

// XXX To be removed
// Boot a full system
func (s *System) Boot() error {
	err := s.BootNamed(fslib.Named())
	if err != nil {
		return err
	}
	err = s.BootFsUxd()
	if err != nil {
		return err
	}
	err = s.BootFss3d()
	if err != nil {
		return err
	}
	err = s.BootProcd()
	if err != nil {
		return err
	}
	return nil
}

// Boot a named and set up the initfs
func (s *System) BootNamed(addr string) error {
	cmd, err := run(s.bin, "/bin/kernel/named", s.namedAddr, []string{"0", addr})
	if err != nil {
		return err
	}
	s.named = cmd
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	s.FsLib = fslib.MakeFsLib("kernel")
	err = named.MakeInitFs(s.FsLib)
	return nil
}

func (s *System) BootFsUxd() error {
	// Create boot cond
	pid := "fsuxd-" + proc.GenPid()
	fsuxdStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	fsuxdStartCond.Init()
	var err error
	fsuxd, err := run(s.bin, "bin/kernel/fsuxd", s.namedAddr, []string{pid})
	s.fsuxd = append(s.fsuxd, fsuxd)
	if err != nil {
		return err
	}
	// Wait for boot
	fsuxdStartCond.Wait()
	return nil
}

func (s *System) BootFss3d() error {
	// Create boot cond
	pid := "fss3d-" + proc.GenPid()
	fss3dStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	fss3dStartCond.Init()
	var err error
	fss3d, err := run(s.bin, "bin/kernel/fss3d", s.namedAddr, []string{pid})
	s.fss3d = append(s.fss3d, fss3d)
	if err != nil {
		return err
	}
	// Wait for boot
	fss3dStartCond.Wait()
	return nil
}

func (s *System) BootProcd() error {
	// Create boot cond
	pid := "procd-" + proc.GenPid()
	procdStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	procdStartCond.Init()
	var err error
	procd, err := run(s.bin, "bin/kernel/procd", s.namedAddr, []string{s.bin, pid})
	s.procd = append(s.procd, procd)
	if err != nil {
		return err
	}
	// Wait for boot
	procdStartCond.Wait()
	return nil
}

func (s *System) RmUnionDir(mdir string) error {
	dirents, err := s.ReadDir(mdir)
	if err != nil {
		return err
	}
	for _, st := range dirents {
		err = s.Remove(mdir + "/" + st.Name + "/")
		if err != nil {
			return err
		}
		err = s.Remove(mdir + "/" + st.Name)
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
			err = syscall.Kill(-s.procd[0].Process.Pid, syscall.SIGKILL)
			//			err = s.procd[0].Process.Kill()
			if err == nil {
				s.procd[0].Wait()
				s.procd = s.procd[1:]
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
	default:
		log.Fatalf("Unkown server type in System.KillOne: %v", srv)
	}
	return nil
}

func (s *System) Shutdown() {
	if len(s.fss3d) != 0 {
		err := s.RmUnionDir(S3)
		if err != nil {
			log.Printf("S3 shutdown %v\n", err)
		}
		for _, d := range s.fss3d {
			d.Wait()
		}
	}
	if len(s.fsuxd) != 0 {
		err := s.RmUnionDir(UX)
		if err != nil {
			log.Printf("Ux shutdown %v\n", err)
		}
		for _, d := range s.fsuxd {
			d.Wait()
		}
	}
	if len(s.procd) != 0 {
		err := s.RmUnionDir(PROCD)
		if err != nil {
			log.Printf("Procds shutdown %v\n", err)
		}
		for _, d := range s.procd {
			d.Wait()
		}
	}

	if s.named != nil {
		// Shutdown named last
		err := s.Remove(NAMED + "/")
		if err != nil {
			// XXX sometimes we get EOF..
			if err.Error() == "EOF" {
				log.Printf("Remove %v shutdown %v\n", NAMED, err)
			} else {
				log.Fatalf("Remove %v shutdown %v\n", NAMED, err)
			}
		}
		s.named.Wait()
	}
}

func run(bin string, name string, namedAddr string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	cmd.Env = append(cmd.Env, "NAMED="+namedAddr)
	return cmd, cmd.Start()
}
