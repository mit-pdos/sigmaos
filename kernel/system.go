package kernel

import (
	"log"
	"net"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	db "ulambda/debug"
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
	SUBSYSTEM_INFO   = "subsystem-info"
)

type System struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	pid         proc.Tpid
	namedAddr   []string
	named       *Subsystem
	fss3d       []*Subsystem
	fsuxd       []*Subsystem
	procd       []*Subsystem
	dbd         []*Subsystem
	crashedPids map[proc.Tpid]bool
}

func makeSystemBase(namedAddr []string) *System {
	s := &System{}
	s.namedAddr = namedAddr
	s.procd = []*Subsystem{}
	s.fsuxd = []*Subsystem{}
	s.fss3d = []*Subsystem{}
	s.dbd = []*Subsystem{}
	s.crashedPids = make(map[proc.Tpid]bool)
	return s
}

func MakeSystemClnt(uname, named string) *System {
	s := makeSystemBase([]string{named})
	s.FsLib = fslib.MakeFsLibAddr(uname, []string{named})
	s.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), s.FsLib, uname, s.namedAddr)
	return s
}

// Make system with just named. replicaId is used to index into the
// fslib.Named() slice and select an address for this named.
func MakeSystemNamed(uname string, replicaId int) *System {
	s := makeSystemBase(fslib.Named())
	// replicaId needs to be 1-indexed for replication library.
	cmd, err := RunNamed(fslib.Named()[replicaId], len(fslib.Named()) > 1, replicaId+1, fslib.Named(), NO_REALM)
	if err != nil {
		db.DFatalf("RunNamed err %v\n", err)
	}
	// XXX It's a bit weird that we set program/pid here...
	proc.SetProgram(uname)
	proc.SetPid(proc.GenPid())
	s.named = makeSubsystem(nil, nil)
	s.named.cmd = cmd
	time.Sleep(SLEEP_MS * time.Millisecond)
	s.FsLib = fslib.MakeFsLibAddr(uname, fslib.Named())
	return s
}

// Make a system with Named and other kernel services
func MakeSystemAll(uname string, replicaId int) *System {
	s := MakeSystemNamed(uname, replicaId)
	// XXX should this be GetPid?
	s.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), s.FsLib, uname, s.namedAddr)
	s.pid = proc.GetPid()
	err := s.Boot()
	if err != nil {
		db.DFatalf("Start err %v\n", err)
	}
	return s
}

func MakeSystem(uname string, namedAddr []string) *System {
	s := makeSystemBase(namedAddr)
	s.FsLib = fslib.MakeFsLibAddr(uname, namedAddr)
	s.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), s.FsLib, uname, namedAddr)
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

func (s *System) BootSubsystem(binpath string, args []string, list *[]*Subsystem) error {
	pid := proc.Tpid(path.Base(binpath) + "-" + proc.GenPid().String())
	p := proc.MakeProcPid(pid, binpath, args)
	ss := makeSubsystem(s.ProcClnt, p)
	*list = append(*list, ss)
	return ss.Run(s.namedAddr)
}

func (s *System) BootFsUxd() error {
	return s.BootSubsystem("kernel/fsuxd", []string{}, &s.fsuxd)
}

func (s *System) BootFss3d() error {
	return s.BootSubsystem("kernel/fss3d", []string{}, &s.fss3d)
}

func (s *System) BootProcd() error {
	return s.BootSubsystem("kernel/procd", []string{}, &s.procd)
}

func (s *System) BootDbd() error {
	return s.BootSubsystem("kernel/dbd", []string{}, &s.dbd)
}

func (s *System) KillOne(srv string) error {
	var err error
	var ss *Subsystem
	switch srv {
	case np.PROCD:
		if len(s.procd) > 0 {
			ss = s.procd[0]
			s.procd = s.procd[1:]
		} else {
			log.Printf("Tried to kill procd, nothing to kill")
		}
	case np.UX:
		if len(s.fsuxd) > 0 {
			ss = s.fsuxd[0]
			s.fsuxd = s.fsuxd[1:]
		} else {
			log.Printf("Tried to kill ux, nothing to kill")
		}
	default:
		db.DFatalf("Unkown server type in System.KillOne: %v", srv)
	}
	log.Printf("kill %v %v\n", -ss.cmd.Process.Pid, ss.p.Pid)
	err = syscall.Kill(-ss.cmd.Process.Pid, syscall.SIGKILL)
	if err == nil {
		ss.cmd.Wait()
		s.crashedPids[ss.p.Pid] = true
	} else {
		db.DFatalf("%v kill failed %v\n", srv, err)
	}
	return nil
}

func (s *System) Shutdown() {
	if s.ProcClnt != nil {
		cpids, err := s.GetChildren()
		if err != nil {
			db.DFatalf("GetChildren in System.Shutdown: %v", err)
		}
		db.DPrintf("KERNEL", "Shutdown children %v", cpids)
		for _, pid := range cpids {
			s.Evict(pid)
			db.DPrintf("KERNEL", "Evicted %v", pid)
			if _, ok := s.crashedPids[pid]; !ok {
				if status, err := s.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
					log.Printf("shutdown error pid %v: %v %v", pid, status, err)
				}
			}
			db.DPrintf("KERNEL", "Done evicting %v", pid)
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
		args = append(args, strings.Join(peers, ","))
	}

	return proc.MakeProcPid(proc.Tpid("pid-"+strconv.Itoa(id)+proc.GenPid().String()), "kernel/named", args)
}

// Run a named (but not as a proc)
func RunNamed(addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := proc.RunKernelProc(p, fslib.Named())
	if err != nil {
		log.Printf("Error running named: %v", err)
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)

	return cmd, nil
}

// Run a named as a proc
func BootNamed(pclnt *procclnt.ProcClnt, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, proc.Tpid, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := pclnt.SpawnKernelProc(p, fslib.Named())
	if err != nil {
		db.DFatalf("Error WaitStart in BootNamed: %v", err)
		return nil, "", err
	}
	if err = pclnt.WaitStart(p.Pid); err != nil {
		db.DFatalf("Error WaitStart in BootNamed: %v", err)
		return nil, "", err
	}
	return cmd, p.Pid, nil
}

func addReplPortOffset(peerAddr string) string {
	// Compute replica address as peerAddr + REPL_PORT_OFFSET
	host, port, err := net.SplitHostPort(peerAddr)
	if err != nil {
		db.DFatalf("Error splitting host port: %v", err)
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		db.DFatalf("Error conv port: %v", err)
	}
	newPort := strconv.Itoa(portI + REPL_PORT_OFFSET)

	return host + ":" + newPort
}
