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
	named *exec.Cmd
	fss3d []*exec.Cmd
	fsuxd []*exec.Cmd
	procd []*exec.Cmd
	*fslib.FsLib
}

func run(bin string, name string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	return cmd, cmd.Start()
}

func BootMin(bin string) (*System, error) {
	s := &System{}
	cmd, err := run(bin, "/bin/kernel/named", []string{"0", ":1111"})
	if err != nil {
		return nil, err
	}
	s.named = cmd
	time.Sleep(POST_BOOT_SLEEP_MS * time.Millisecond)
	s.FsLib = fslib.MakeFsLib("kernel")
	err = named.MakeInitFs(s.FsLib)
	return s, nil
}

func Boot(bin string) (*System, error) {
	s, err := BootMin(bin)
	if err != nil {
		return nil, err
	}
	err = s.BootFsUxd(bin)
	if err != nil {
		return nil, err
	}
	err = s.BootFss3d(bin)
	if err != nil {
		return nil, err
	}
	err = s.BootProcd(bin)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *System) BootFsUxd(bin string) error {
	// Create boot cond
	pid := "fsuxd-" + proc.GenPid()
	fsuxdStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	fsuxdStartCond.Init()
	var err error
	fsuxd, err := run(bin, "bin/kernel/fsuxd", []string{pid})
	s.fsuxd = append(s.fsuxd, fsuxd)
	if err != nil {
		return err
	}
	// Wait for boot
	fsuxdStartCond.Wait()
	return nil
}

func (s *System) BootFss3d(bin string) error {
	// Create boot cond
	pid := "fss3d-" + proc.GenPid()
	fss3dStartCond := sync.MakeCond(s.FsLib, path.Join(BOOT, pid), nil)
	fss3dStartCond.Init()
	var err error
	fss3d, err := run(bin, "bin/kernel/fss3d", []string{pid})
	s.fss3d = append(s.fss3d, fss3d)
	if err != nil {
		return err
	}
	// Wait for boot
	fss3dStartCond.Wait()
	return nil
}

func (s *System) BootProcd(bin string) error {
	// Create boot cond
	pid := "procd-" + proc.GenPid()
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
	}
	return nil
}

func (s *System) Shutdown(clnt *fslib.FsLib) {
	if len(s.fss3d) != 0 {
		err := s.RmUnionDir(clnt, S3)
		if err != nil {
			log.Printf("S3 shutdown %v\n", err)
		}
		for _, d := range s.fss3d {
			d.Wait()
		}
	}
	if len(s.fsuxd) != 0 {
		err := s.RmUnionDir(clnt, UX)
		if err != nil {
			log.Printf("Ux shutdown %v\n", err)
		}
		for _, d := range s.fsuxd {
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
