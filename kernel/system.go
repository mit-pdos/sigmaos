package kernel

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	NO_REALM = "no-realm"
	SLEEP_MS = 100
)

type System struct {
	bin              string
	pid              string
	namedAddr        []string
	named            *exec.Cmd
	fss3d            []*exec.Cmd
	fsuxd            []*exec.Cmd
	procd            []*exec.Cmd
	procdPids        []string
	crashedProcdPids map[string]bool
	dbd              []*exec.Cmd
	*procclnt.ProcClnt
}

func MakeSystem(bin string, fsl *fslib.FsLib, namedAddr []string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = namedAddr
	s.ProcClnt = procclnt.MakeProcClntInit(fsl, namedAddr)
	s.pid = proc.GetPid()
	s.procdPids = []string{}
	s.crashedProcdPids = make(map[string]bool)
	return s
}

// Make system with just named
func MakeSystemNamed(bin string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = fslib.Named()
	cmd, err := BootNamed(s.bin, fslib.NamedAddr(), false, 0, nil, NO_REALM)
	if err != nil {
		return nil
	}
	s.named = cmd
	time.Sleep(SLEEP_MS * time.Millisecond)
	s.procdPids = []string{}
	s.crashedProcdPids = make(map[string]bool)
	return s
}

func MakeSystemAll(uname, bin string) (*System, *fslib.FsLib, error) {
	s := MakeSystemNamed(bin)
	fsl := fslib.MakeFsLibAddr(uname, fslib.Named())
	err := s.Start(fsl)
	return s, fsl, err
}

// Start kernel services
func (s *System) Start(fsl *fslib.FsLib) error {
	s.ProcClnt = procclnt.MakeProcClntInit(fsl, s.namedAddr)
	s.pid = proc.GetPid()
	return s.Boot()
}

// Boot a "kernel" without named
func (s *System) Boot() error {
	if err := s.BootFsUxd(); err != nil {
		return err
	}
	if err := s.BootFss3d(); err != nil {
		return err
	}
	if err := s.BootProcd(); err != nil {
		return err
	}
	if err := s.BootDbd(); err != nil {
		return err
	}
	return nil
}

func (s *System) BootFsUxd() error {
	p := proc.MakeProcPid("fsuxd-"+proc.GenPid(), "bin/kernel/fsuxd", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bin, s.namedAddr)
	if err != nil {
		return err
	}
	s.fsuxd = append(s.fsuxd, cmd)
	return s.WaitStart(p.Pid)
}

func (s *System) BootFss3d() error {
	p := proc.MakeProcPid("fss3d-"+proc.GenPid(), "bin/kernel/fss3d", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bin, s.namedAddr)
	if err != nil {
		return err
	}
	s.fss3d = append(s.fss3d, cmd)
	return s.WaitStart(p.Pid)
}

func (s *System) BootProcd() error {
	p := proc.MakeProcPid("procd-"+proc.GenPid(), "bin/kernel/procd", []string{s.bin})
	cmd, err := s.SpawnKernelProc(p, s.bin, s.namedAddr)
	if err != nil {
		return err
	}
	s.procdPids = append(s.procdPids, p.Pid)
	s.procd = append(s.procd, cmd)
	return s.WaitStart(p.Pid)
}

func (s *System) BootDbd() error {
	p := proc.MakeProcPid("dbd-"+proc.GenPid(), "bin/kernel/dbd", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bin, s.namedAddr)
	if err != nil {
		return err
	}
	s.dbd = append(s.dbd, cmd)
	return s.WaitStart(p.Pid)
}

func (s *System) KillOne(srv string) error {
	var err error
	switch srv {
	case np.PROCD:
		if len(s.procd) > 0 {
			log.Printf("kill %v\n", -s.procd[0].Process.Pid)
			err = syscall.Kill(-s.procd[0].Process.Pid, syscall.SIGKILL)
			//			err = s.procd[0].Process.Kill()
			if err == nil {
				s.procd[0].Wait()
				s.procd = s.procd[1:]
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
		s.crashedProcdPids[s.procdPids[0]] = true
		s.procdPids = s.procdPids[1:]
	default:
		log.Fatalf("Unkown server type in System.KillOne: %v", srv)
	}
	return nil
}

func (s *System) Shutdown(fsl *fslib.FsLib) {
	if s.ProcClnt != nil {
		cpids, err := s.GetChildren(s.pid)
		if err != nil {
			log.Fatalf("Error GetChildren in System.Shutdown: %v", err)
		}
		for _, pid := range cpids {
			s.Evict(pid)
			if _, ok := s.crashedProcdPids[pid]; !ok {
				if status, err := s.WaitExit(pid); status != "EVICTED" || err != nil {
					log.Printf("shutdown error %v %v", status, err)
				}
			}
		}
	}
	// Make sure the procs actually exited
	for _, d := range s.fss3d {
		d.Wait()
	}
	for _, d := range s.fsuxd {
		d.Wait()
	}
	for _, d := range s.procd {
		d.Wait()
	}
	for _, d := range s.dbd {
		d.Wait()
	}
	if s.named != nil {
		err := fsl.ShutdownFs(np.NAMED)
		if err != nil {
			log.Printf("Named shutdown %v\n", err)
		}
		s.named.Wait()
	}
}

// Boot a named
func BootNamed(bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	args := []string{addr, realmId}
	// If we're running replicated...
	if replicate {
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	cmd, err := proc.Run("named-"+strconv.Itoa(id), bin, "/bin/kernel/named", fslib.Named(), args)
	if err != nil {
		log.Printf("Error running named: %v", err)
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)

	return cmd, nil
}

// Run a named as a proc
func RunNamed(pclnt *procclnt.ProcClnt, bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, string, error) {
	args := []string{addr, realmId}
	// If we're running replicated...
	if replicate {
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	pid := "named-" + strconv.Itoa(id)
	p := proc.MakeProcPid(pid, "bin/kernel/named", args)
	cmd, err := pclnt.SpawnKernelProc(p, bin, fslib.Named())
	if err != nil {
		log.Fatalf("Error WaitStart in RunNamed: %v", err)
		return nil, "", err
	}
	if err = pclnt.WaitStart(p.Pid); err != nil {
		log.Fatalf("Error WaitStart in RunNamed: %v", err)
		return nil, "", err
	}
	return cmd, pid, nil
}
