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
	SLEEP_MS = 1000
)

type System struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	bindir      string
	pid         string
	namedAddr   []string
	named       *exec.Cmd
	fss3d       []*exec.Cmd
	fsuxd       []*exec.Cmd
	procd       []*exec.Cmd
	procdPids   []string
	fsuxdPids   []string
	crashedPids map[string]bool
	dbd         []*exec.Cmd
}

func makeSystemBase(namedAddr []string, bindir string) *System {
	s := &System{}
	s.bindir = bindir
	s.namedAddr = namedAddr
	s.procdPids = []string{}
	s.fsuxdPids = []string{}
	s.crashedPids = make(map[string]bool)
	return s
}

// Make system with just named. replicaId is used to index into the
// fslib.Named() slice and select an address for this named.
func MakeSystemNamed(uname, bin string, replicaId int) *System {
	s := makeSystemBase(fslib.Named(), bin)
	// replicaId needs to be 1-indexed for replication library.
	cmd, err := RunNamed(s.bindir, fslib.Named()[replicaId], len(fslib.Named()) > 1, replicaId+1, fslib.Named(), NO_REALM)
	if err != nil {
		log.Fatalf("FATAL RunNamed err %v\n", err)
	}
	s.named = cmd
	time.Sleep(SLEEP_MS * time.Millisecond)
	s.FsLib = fslib.MakeFsLibAddr(uname, fslib.Named())
	return s
}

// Make a system with Named and other kernel services
func MakeSystemAll(uname, bin string, replicaId int) *System {
	s := MakeSystemNamed(uname, bin, replicaId)
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, uname, s.namedAddr)
	s.pid = proc.GetPid()
	err := s.Boot()
	if err != nil {
		log.Fatalf("FATAL Start err %v\n", err)
	}
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
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.fsuxd = append(s.fsuxd, cmd)
	s.fsuxdPids = append(s.fsuxdPids, p.Pid)
	return s.WaitStart(p.Pid)
}

func (s *System) BootFss3d() error {
	p := proc.MakeProcPid("fss3d-"+proc.GenPid(), "bin/kernel/fss3d", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.fss3d = append(s.fss3d, cmd)
	return s.WaitStart(p.Pid)
}

func (s *System) BootProcd() error {
	p := proc.MakeProcPid("procd-"+proc.GenPid(), "bin/kernel/procd", []string{s.bindir})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.procdPids = append(s.procdPids, p.Pid)
	s.procd = append(s.procd, cmd)
	return s.WaitStart(p.Pid)
}

func (s *System) BootDbd() error {
	p := proc.MakeProcPid("dbd-"+proc.GenPid(), "bin/kernel/dbd", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
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
			if err == nil {
				s.procd[0].Wait()
				s.procd = s.procd[1:]
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
		s.crashedPids[s.procdPids[0]] = true
		s.procdPids = s.procdPids[1:]
	case np.UX:
		log.Printf("kill %v\n", -s.fsuxd[0].Process.Pid)
		err = syscall.Kill(-s.fsuxd[0].Process.Pid, syscall.SIGKILL)
		if err == nil {
			s.fsuxd[0].Wait()
			s.fsuxd = s.fsuxd[1:]
			s.crashedPids[s.fsuxdPids[0]] = true
			s.fsuxdPids = s.fsuxdPids[1:]
		} else {
			log.Fatalf("Ux kill failed %v\n", err)
		}
	default:
		log.Fatalf("Unkown server type in System.KillOne: %v", srv)
	}
	return nil
}

func (s *System) Shutdown() {
	if s.ProcClnt != nil {
		cpids, err := s.GetChildren(proc.GetProcDir())
		if err != nil {
			log.Fatalf("FATAL GetChildren in System.Shutdown: %v", err)
		}
		for _, pid := range cpids {
			s.Evict(pid)
			if _, ok := s.crashedPids[pid]; !ok {
				if status, err := s.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
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
		err := s.ShutdownFs(np.NAMED)
		if err != nil {
			log.Printf("Named shutdown %v\n", err)
			// forcefully kill it so that test terminates
			s.named.Process.Kill()
		}
		s.named.Wait()
	}
}

// Run a named (but not as a proc)
func RunNamed(bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	args := []string{addr, realmId}
	// If we're running replicated...
	if replicate {
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	p := proc.MakeProcPid("pid-"+strconv.Itoa(id), "/bin/kernel/named", args)

	cmd, err := proc.RunKernelProc(p, bin, fslib.Named())
	if err != nil {
		log.Printf("Error running named: %v", err)
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)

	return cmd, nil
}

func MakeSystem(uname, bin string, namedAddr []string) *System {
	s := makeSystemBase(namedAddr, bin)
	s.FsLib = fslib.MakeFsLibAddr(uname, namedAddr)
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, uname, namedAddr)
	s.pid = proc.GetPid()
	return s
}

// Run a named as a proc
func BootNamed(pclnt *procclnt.ProcClnt, bindir string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, string, error) {
	args := []string{addr, realmId}
	// If we're running replicated...
	if replicate {
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	pid := "pid-" + strconv.Itoa(id)
	p := proc.MakeProcPid(pid, "bin/kernel/named", args)
	cmd, err := pclnt.SpawnKernelProc(p, bindir, fslib.Named())
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
