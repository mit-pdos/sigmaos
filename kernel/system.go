package kernel

import (
	"log"
	"net"
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
	NO_PID           = "no-realm"
	NO_REALM         = "no-realm"
	SLEEP_MS         = 200
	REPL_PORT_OFFSET = 100
)

type System struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	bindir      string
	pid         string
	namedAddr   []string
	named       *Subsystem
	fss3d       []*Subsystem
	fsuxd       []*Subsystem
	procd       []*Subsystem
	dbd         []*Subsystem
	crashedPids map[string]bool
}

func makeSystemBase(namedAddr []string, bindir string) *System {
	s := &System{}
	s.bindir = bindir
	s.namedAddr = namedAddr
	s.procd = []*Subsystem{}
	s.fsuxd = []*Subsystem{}
	s.fss3d = []*Subsystem{}
	s.dbd = []*Subsystem{}
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
	proc.SetProgram(uname)
	proc.SetPid(proc.GenPid())
	s.named = makeSubsystem(cmd, "init-named")
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

func MakeSystem(uname, bin string, namedAddr []string) *System {
	s := makeSystemBase(namedAddr, bin)
	s.FsLib = fslib.MakeFsLibAddr(uname, namedAddr)
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, uname, namedAddr)
	s.pid = proc.GetPid()
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
	p := proc.MakeProcPid(proc.GenPid(), "bin/kernel/fsuxd", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.fsuxd = append(s.fsuxd, makeSubsystem(cmd, p.Pid))
	return s.WaitStart(p.Pid)
}

func (s *System) BootFss3d() error {
	p := proc.MakeProcPid(proc.GenPid(), "bin/kernel/fss3d", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.fss3d = append(s.fss3d, makeSubsystem(cmd, p.Pid))
	return s.WaitStart(p.Pid)
}

func (s *System) BootProcd() error {
	p := proc.MakeProcPid(proc.GenPid(), "bin/kernel/procd", []string{s.bindir})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.procd = append(s.procd, makeSubsystem(cmd, p.Pid))
	return s.WaitStart(p.Pid)
}

func (s *System) BootDbd() error {
	p := proc.MakeProcPid(proc.GenPid(), "bin/kernel/dbd", []string{})
	cmd, err := s.SpawnKernelProc(p, s.bindir, s.namedAddr)
	if err != nil {
		return err
	}
	s.dbd = append(s.dbd, makeSubsystem(cmd, p.Pid))
	return s.WaitStart(p.Pid)
}

func (s *System) KillOne(srv string) error {
	var err error
	switch srv {
	case np.PROCD:
		if len(s.procd) > 0 {
			log.Printf("kill %v\n", -s.procd[0].cmd.Process.Pid, s.procd[0].pid)
			err = syscall.Kill(-s.procd[0].cmd.Process.Pid, syscall.SIGKILL)
			if err == nil {
				s.procd[0].cmd.Wait()
				s.crashedPids[s.procd[0].pid] = true
				s.procd = s.procd[1:]
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
	case np.UX:
		log.Printf("kill %v\n", -s.fsuxd[0].cmd.Process.Pid)
		err = syscall.Kill(-s.fsuxd[0].cmd.Process.Pid, syscall.SIGKILL)
		if err == nil {
			s.fsuxd[0].cmd.Wait()
			s.crashedPids[s.fsuxd[0].pid] = true
			s.fsuxd = s.fsuxd[1:]
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
		d.cmd.Wait()
	}
	for _, d := range s.fsuxd {
		d.cmd.Wait()
	}
	for _, d := range s.procd {
		d.cmd.Wait()
	}
	for _, d := range s.dbd {
		d.cmd.Wait()
	}
	if s.named != nil {
		// kill it so that test terminates
		s.named.cmd.Process.Kill()
		s.named.cmd.Wait()
	}
}

func makeNamedProc(addr string, replicate bool, id int, pe []string, realmId string) *proc.Proc {
	args := []string{addr, realmId}
	// If we're running replicated...
	if replicate {
		// Add an offset to the peers' port addresses.
		peers := []string{}
		for _, peer := range pe {
			peers = append(peers, addReplPortOffset(peer))
		}
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	return proc.MakeProcPid("pid-"+strconv.Itoa(id), "/bin/kernel/named", args)
}

// Run a named (but not as a proc)
func RunNamed(bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := proc.RunKernelProc(p, bin, fslib.Named())
	if err != nil {
		log.Printf("Error running named: %v", err)
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)

	return cmd, nil
}

// Run a named as a proc
func BootNamed(pclnt *procclnt.ProcClnt, bindir string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, string, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := pclnt.SpawnKernelProc(p, bindir, fslib.Named())
	if err != nil {
		log.Fatalf("Error WaitStart in BootNamed: %v", err)
		return nil, "", err
	}
	if err = pclnt.WaitStart(p.Pid); err != nil {
		log.Fatalf("Error WaitStart in BootNamed: %v", err)
		return nil, "", err
	}
	return cmd, p.Pid, nil
}

func addReplPortOffset(peerAddr string) string {
	// Compute replica address as peerAddr + REPL_PORT_OFFSET
	host, port, err := net.SplitHostPort(peerAddr)
	if err != nil {
		log.Fatalf("Error splitting host port: %v", err)
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		log.Fatalf("Error conv port: %v", err)
	}
	newPort := strconv.Itoa(portI + REPL_PORT_OFFSET)

	return host + ":" + newPort
}
