package kernel

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/sync"
)

// XXX move to named
const ()

type System struct {
	bin       string
	namedAddr []string
	fss3d     []*exec.Cmd
	fsuxd     []*exec.Cmd
	procd     []*exec.Cmd
	dbd       []*exec.Cmd
	*fslib.FsLib
}

// XXX To be removed
func MakeSystem(bin string) *System {
	return MakeSystemNamedAddr(bin, fslib.Named())
}

func MakeSystemNamedAddr(bin string, namedAddr []string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = namedAddr
	s.FsLib = fslib.MakeFsLibAddr("kernel", namedAddr)
	return s
}

// Boot a full system
func (s *System) Boot() error {
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
	err = s.BootDbd()
	if err != nil {
		return err
	}
	return nil
}

func (s *System) BootFsUxd() error {
	// Create boot cond
	pid := "fsuxd-" + proc.GenPid()
	fsuxdStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil)
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
	fss3dStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil)
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
	procdStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil)
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

func (s *System) BootDbd() error {
	// Create dbd cond
	pid := "dbd-" + proc.GenPid()
	dbdStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil)
	dbdStartCond.Init()
	var err error
	dbd, err := run(s.bin, "bin/kernel/dbd", s.namedAddr, []string{pid})
	s.dbd = append(s.dbd, dbd)
	if err != nil {
		return err
	}
	// Wait for boot
	dbdStartCond.Wait()
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
	case named.PROCD:
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
		err := s.RmUnionDir(named.S3)
		if err != nil {
			log.Printf("named.S3 shutdown %v\n", err)
		}
		for _, d := range s.fss3d {
			d.Wait()
		}
	}
	if len(s.fsuxd) != 0 {
		err := s.RmUnionDir(named.UX)
		if err != nil {
			log.Printf("Ux shutdown %v\n", err)
		}
		for _, d := range s.fsuxd {
			d.Wait()
		}
	}
	if len(s.procd) != 0 {
		err := s.RmUnionDir(named.PROCD)
		if err != nil {
			log.Printf("Procds shutdown %v\n", err)
		}
		for _, d := range s.procd {
			d.Wait()
		}
	}
	if len(s.dbd) != 0 {
		err := s.RmUnionDir(named.DB)
		if err != nil {
			log.Printf("Db shutdown %v\n", err)
		}
		for _, d := range s.dbd {
			d.Wait()
		}
	}
}

func run(bin string, name string, namedAddr []string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	cmd.Env = append(cmd.Env, "NAMED="+strings.Join(namedAddr, ","))
	return cmd, cmd.Start()
}
