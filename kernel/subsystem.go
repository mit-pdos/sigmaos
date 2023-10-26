package kernel

import (
	"fmt"
	"os/exec"
	"syscall"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelsubinfo"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

const NPORT_PER_CONTAINER = 20

type Subsystem struct {
	*procclnt.ProcClnt
	k         *Kernel
	p         *proc.Proc
	how       proc.Thow
	cmd       *exec.Cmd
	container *container.Container
	waited    bool
	crashed   bool
}

func (ss *Subsystem) String() string {
	s := fmt.Sprintf("subsystem %p: [proc %v how %v]", ss, ss.p, ss.how)
	return s
}

func newSubsystemCmd(pclnt *procclnt.ProcClnt, k *Kernel, p *proc.Proc, how proc.Thow, cmd *exec.Cmd) *Subsystem {
	return &Subsystem{pclnt, k, p, how, cmd, nil, false, false}
}

func newSubsystem(pclnt *procclnt.ProcClnt, k *Kernel, p *proc.Proc, how proc.Thow) *Subsystem {
	return newSubsystemCmd(pclnt, k, p, how, nil)
}

func (k *Kernel) bootSubsystemWithMcpu(program string, args []string, how proc.Thow, mcpu proc.Tmcpu) (*Subsystem, error) {
	pid := sp.GenPid(program)
	p := proc.NewPrivProcPid(pid, program, args, true)
	p.SetMcpu(mcpu)
	ss := newSubsystem(k.ProcClnt, k, p, how)
	return ss, ss.Run(how, k.Param.KernelId, k.ip)
}

func (k *Kernel) bootSubsystem(program string, args []string, how proc.Thow) (*Subsystem, error) {
	return k.bootSubsystemWithMcpu(program, args, how, 0)
}

func (s *Subsystem) Run(how proc.Thow, kernelId, localIP string) error {
	if how == proc.HLINUX || how == proc.HSCHEDD {
		cmd, err := s.SpawnKernelProc(s.p, s.how, kernelId)
		if err != nil {
			return err
		}
		s.cmd = cmd
	} else {
		realm := sp.Trealm(s.p.Args[0])
		ptype := proc.ParseTtype(s.p.Args[1])
		if err := s.NewProc(s.p, proc.HDOCKER, kernelId); err != nil {
			return err
		}
		// XXX don't hard code
		h := sp.SIGMAHOME
		s.p.AppendEnv("PATH", h+"/bin/user:"+h+"/bin/user/common:"+h+"/bin/kernel:/usr/sbin:/usr/bin:/bin")
		s.p.FinalizeEnv(localIP, sp.Tpid(proc.NOT_SET))
		var r *port.Range
		up := port.NOPORT
		if s.k.Param.Overlays {
			r = &port.Range{FPORT, LPORT}
			up = r.Fport
		}
		c, err := container.StartPContainer(s.p, kernelId, realm, r, up, ptype)
		if err != nil {
			return err
		}
		s.container = c
	}
	err := s.WaitStartKernelProc(s.p.GetPid(), how)
	return err
}

func (ss *Subsystem) SetCPUShares(shares int64) error {
	return ss.container.SetCPUShares(shares)
}

func (ss *Subsystem) GetCPUUtil() (float64, error) {
	return ss.container.GetCPUUtil()
}

func (ss *Subsystem) AllocPort(p port.Tport) (*port.PortBinding, error) {
	if p == port.NOPORT {
		return ss.container.AllocPort()
	} else {
		return ss.container.AllocPortOne(p)
	}
}

func (ss *Subsystem) GetIp(fsl *fslib.FsLib) string {
	return kernelsubinfo.GetSubsystemInfo(fsl, sp.KPIDS, ss.p.GetPid().String()).Ip
}

// Send SIGTERM to a system.
func (s *Subsystem) Terminate() error {
	db.DPrintf(db.KERNEL, "Terminate %v %v\n", s.cmd.Process.Pid, s.cmd)
	if s.how != proc.HLINUX {
		db.DFatalf("Tried to terminate a kernel subsystem spawned through procd: %v", s.p)
	}
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGTERM)
}

// Kill a subsystem, either by sending SIGKILL or Evicting it.
func (s *Subsystem) Kill() error {
	s.crashed = true
	db.DPrintf(db.KERNEL, "Kill %v\n", s)
	if s.p.GetProgram() == "knamed" {
		return stopKNamed(s.cmd)
	}
	if s.how == proc.HSCHEDD || s.how == proc.HDOCKER {
		db.DPrintf(db.ALWAYS, "Killing a kernel subsystem spawned through %v: %v", s.p, s.how)
		err := s.EvictKernelProc(s.p.GetPid(), s.how)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error killing procd-spawned kernel proc: %v err %v", s.p.GetPid(), err)
		}
		return err
	}
	db.DPrintf(db.ALWAYS, "kill %v\n", s.cmd.Process.Pid)
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGKILL)
}

func (s *Subsystem) Wait() error {
	db.DPrintf(db.KERNEL, "Wait for %v to terminate\n", s)
	if s.how == proc.HSCHEDD || s.how == proc.HDOCKER {
		if !s.waited {
			// Only wait if this proc has not been waited for already, since calling
			// WaitExit twice leads to an error.
			status, err := s.WaitExitKernelProc(s.p.GetPid(), s.how)
			if err != nil || !status.IsStatusOK() {
				db.DPrintf(db.ALWAYS, "Subsystem exit with status %v err %v", status, err)
				return err
			}
		}
		return nil
	} else {
		if err := s.cmd.Wait(); err != nil {
			return err
		}
	}
	if s.container != nil {
		return s.container.Shutdown()
	}
	return nil
}
