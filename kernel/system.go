package kernel

import (
	"log"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/sync"
)

const (
	NO_REALM = "no-realm"
	SLEEP_MS = 100
)

type System struct {
	bin       string
	namedAddr []string
	named     *exec.Cmd
	fss3d     []*exec.Cmd
	fsuxd     []*exec.Cmd
	procd     []*exec.Cmd
	dbd       []*exec.Cmd
	*procclnt.ProcClnt
	*fslib.FsLib
}

func MakeSystem(bin string, namedAddr []string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = namedAddr
	s.FsLib = fslib.MakeFsLibAddr("kernel", namedAddr)
	_ = s.Mkdir(named.PROCD, 0777)
	//	proc.SetPid("kernel-" + proc.GenPid())
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, namedAddr)
	return s
}

// Make system with just named
func MakeSystemNamed(bin string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = fslib.Named()
	cmd, err := BootNamed(nil, s.bin, fslib.NamedAddr(), false, 0, nil, NO_REALM)
	if err != nil {
		return nil
	}
	s.named = cmd
	time.Sleep(10 * time.Millisecond)
	s.FsLib = fslib.MakeFsLibAddr("kernel", s.namedAddr)
	_ = s.Mkdir(named.PROCD, 0777)
	s.ProcClnt = procclnt.MakeProcClntInit(s.FsLib, s.namedAddr)
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
	default:
		log.Fatalf("Unkown server type in System.KillOne: %v", srv)
	}
	return nil
}

func (s *System) Shutdown() {
	cpids, err := s.GetChildren(proc.GetPid())
	if err != nil {
		log.Fatalf("Error GetChildren in System.Shutdown: %v", err)
	}
	_ = cpids
	// XXX eviction signals are sometimes dropped at the moment, so just kill processes
	//	for _, pid := range cpids {
	//		log.Printf("pid pre: %v", pid)
	//		s.Evict(pid)
	//		if status, err := s.WaitExit(pid); status != "EVICTED" || err != nil {
	//			log.Printf("%v shutdown error %v %v", status, err)
	//		}
	//		log.Printf("pid post: %v", pid)
	//	}
	// Make sure the procs actually exited
	for _, d := range s.fss3d {
		d.Process.Kill() // XXX remove
		d.Wait()
	}
	for _, d := range s.fsuxd {
		d.Process.Kill() // XXX remove
		d.Wait()
	}
	for _, d := range s.procd {
		d.Process.Kill() // XXX remove
		d.Wait()
	}
	for _, d := range s.dbd {
		d.Process.Kill() // XXX remove
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
func BootNamed(rootFsl *fslib.FsLib, bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
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

	// If this isn't the root named, create a cond to wait on
	var namedStartCond *sync.Cond
	if rootFsl != nil {
		namedStartCond = sync.MakeCond(rootFsl, path.Join(named.BOOT, addr), nil, true)
		namedStartCond.Init()
	}

	cmd, err := proc.Run("named-"+strconv.Itoa(id), bin, "/bin/kernel/named", fslib.Named(), args)
	if err != nil {
		log.Printf("Error running named: %v", err)
		return nil, err
	}

	if rootFsl != nil {
		namedStartCond.Wait()
	} else {
		time.Sleep(SLEEP_MS * time.Millisecond)
	}

	fsl := fslib.MakeFsLibAddr("realm", []string{addr})
	if err := named.MakeInitFs(fsl); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Printf("MakeInitFs error: %v", err)
		return nil, err
	}
	return cmd, nil
}
