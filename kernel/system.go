package kernel

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
	"ulambda/named"
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
	*fslib.FsLib
}

func MakeSystem(bin string, namedAddr []string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = namedAddr
	s.FsLib = fslib.MakeFsLibAddr("kernel", namedAddr)
	//	proc.SetPid("kernel-" + proc.GenPid())
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, namedAddr)
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
	cmd, _, err := BootNamed(nil, s.bin, fslib.NamedAddr(), false, 0, nil, NO_REALM)
	if err != nil {
		return nil
	}
	s.named = cmd
	time.Sleep(SLEEP_MS * time.Millisecond)
	s.FsLib = fslib.MakeFsLibAddr("kernel", s.namedAddr)
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, s.namedAddr)
	s.pid = proc.GetPid()
	s.procdPids = []string{}
	s.crashedProcdPids = make(map[string]bool)
	return s
}

// Make a system with all kernel services
func MakeSystemAll(bin string) *System {
	s := MakeSystemNamed(bin)
	s.Boot()
	return s
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
		s.crashedProcdPids[s.procdPids[0]] = true
		s.procdPids = s.procdPids[1:]
	default:
		log.Fatalf("Unkown server type in System.KillOne: %v", srv)
	}
	return nil
}

func (s *System) Shutdown() {
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
		err := s.ShutdownFs(named.NAMED)
		if err != nil {
			log.Printf("Named shutdown %v\n", err)
		}
		s.named.Wait()
	}
}

// Boot a named and set up the initfs
func BootNamed(pclnt *procclnt.ProcClnt, bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, string, error) {
	var args []string
	if realmId == NO_REALM {
		args = []string{addr, NO_REALM}
	} else {
		args = []string{addr, realmId}
	}
	// If we're running replicated...
	if replicate {
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	// If this isn't a root named, spawn it. Else, just run it directly.
	var cmd *exec.Cmd
	var pid string
	var err error
	if pclnt == nil {
		cmd, err = proc.Run("named-"+strconv.Itoa(id), bin, "/bin/kernel/named", fslib.Named(), args)
		if err != nil {
			log.Printf("Error running named: %v", err)
			return nil, "", err
		}
		time.Sleep(SLEEP_MS * time.Millisecond)
	} else {
		pid = "named-" + strconv.Itoa(id)
		p := proc.MakeProcPid(pid, "bin/kernel/named", args)
		cmd, err = pclnt.SpawnKernelProc(p, bin, fslib.Named())
		if err != nil {
			log.Fatalf("Error WaitStart in BootNamed: %v", err)
			return nil, "", err
		}
		if err = pclnt.WaitStart(p.Pid); err != nil {
			log.Fatalf("Error WaitStart in BootNamed: %v", err)
			return nil, "", err
		}
	}

	fsl := fslib.MakeFsLibAddr("realm", []string{addr})
	if err := named.MakeInitFs(fsl); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Printf("MakeInitFs error: %v", err)
		return nil, "", err
	}
	return cmd, pid, nil
}
