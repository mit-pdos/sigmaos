package kernel

import (
	"fmt"
	"os/exec"
	"syscall"

	"sigmaos/dcontainer"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type Subsystem interface {
	GetProc() *proc.Proc
	GetCrashed() bool
	Evict() error
	Wait() error
	Kill() error
	SetCPUShares(shares int64) error
	GetCPUUtil() (float64, error)
	Run(how proc.Thow, kernelId string, localIP sp.Tip) error
}

type KernelSubsystem struct {
	*sigmaclnt.SigmaClntKernel
	k         *Kernel
	p         *proc.Proc
	how       proc.Thow
	cmd       *exec.Cmd
	container *dcontainer.DContainer
	crashed   bool
}

func (ss *KernelSubsystem) GetProc() *proc.Proc {
	return ss.p
}

func (ss *KernelSubsystem) GetCrashed() bool {
	return ss.crashed
}

func (ss *KernelSubsystem) String() string {
	s := fmt.Sprintf("subsystem %p: [proc %v how %v]", ss, ss.p, ss.how)
	return s
}

func newSubsystemCmd(sc *sigmaclnt.SigmaClntKernel, k *Kernel, p *proc.Proc, how proc.Thow, cmd *exec.Cmd) Subsystem {
	return &KernelSubsystem{sc, k, p, how, cmd, nil, false}
}

func newSubsystem(sc *sigmaclnt.SigmaClntKernel, k *Kernel, p *proc.Proc, how proc.Thow) Subsystem {
	return newSubsystemCmd(sc, k, p, how, nil)
}

func (k *Kernel) bootSubsystemPIDWithMcpu(pid sp.Tpid, program string, args, env []string, realm sp.Trealm, how proc.Thow, mcpu proc.Tmcpu) (Subsystem, error) {
	p := proc.NewPrivProcPid(pid, program, args, true)
	p.SetRealm(realm)
	p.GetProcEnv().SetInnerContainerIP(k.ip)
	p.GetProcEnv().SetOuterContainerIP(k.ip)
	p.GetProcEnv().SetSecrets(k.ProcEnv().GetSecrets())
	p.SetMcpu(mcpu)
	p.UpdateEnv(env)
	var sck *sigmaclnt.SigmaClntKernel
	var err error
	if realm == sp.ROOTREALM {
		sck = k.SigmaClntKernel
	} else {
		sck, err = k.getRealmSigmaClnt(realm)
		if err != nil {
			return nil, err
		}
	}
	ss := newSubsystem(sck, k, p, how)
	return ss, ss.Run(how, k.Param.KernelID, k.ip)
}

func (k *Kernel) bootSubsystem(program string, args, env []string, realm sp.Trealm, how proc.Thow, mcpu proc.Tmcpu) (Subsystem, error) {
	pid := sp.GenPidKernelProc(program, k.k.Param.KernelID)
	return k.bootSubsystemPIDWithMcpu(pid, program, args, env, realm, how, mcpu)
}

func (s *KernelSubsystem) Evict() error {
	return s.EvictKernelProc(s.p.GetPid(), s.how)
}

func (s *KernelSubsystem) Run(how proc.Thow, kernelId string, localIP sp.Tip) error {
	if how == proc.HLINUX || how == proc.HMSCHED {
		cmd, err := s.SpawnKernelProc(s.p, s.how, kernelId)
		if err != nil {
			return err
		}
		s.cmd = cmd
	} else {
		if err := s.NewProc(s.p, proc.HDOCKER, kernelId); err != nil {
			return err
		}
		h := sp.SIGMAHOME
		s.p.AppendEnv("PATH", h+"/bin/user:"+h+"/bin/user/common:"+h+"/bin/kernel:/usr/sbin:/usr/bin:/bin")
		s.p.FinalizeEnv(localIP, localIP, sp.Tpid(sp.NOT_SET))
		c, err := dcontainer.StartDockerContainer(s.p, kernelId, s.k.Param.User, s.k.Param.Net)
		if err != nil {
			return err
		}
		s.container = c
	}
	err := s.WaitStartKernelProc(s.p.GetPid(), how)
	return err
}

func (ss *KernelSubsystem) SetCPUShares(shares int64) error {
	return ss.container.SetCPUShares(shares)
}

func (ss *KernelSubsystem) GetCPUUtil() (float64, error) {
	return ss.container.GetCPUUtil()
}

// Send SIGTERM to a system.
func (s *KernelSubsystem) Terminate() error {
	db.DPrintf(db.KERNEL, "Terminate %v %v\n", s.cmd.Process.Pid, s.cmd)
	if s.how != proc.HLINUX {
		db.DPrintf(db.ERROR, "Tried to terminate a kernel subsystem spawned through procd: %v", s.p)
		return fmt.Errorf("Tried to terminate a kernel subsystem spawned through procd: %v", s.p)
	}
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGTERM)
}

// Kill a subsystem, either by sending SIGKILL or Evicting it.
func (s *KernelSubsystem) Kill() error {
	s.crashed = true
	db.DPrintf(db.KERNEL, "Kill %v\n", s)
	if s.p.GetProgram() == "knamed" {
		return stopKNamed(s.cmd)
	}
	if s.how == proc.HMSCHED || s.how == proc.HDOCKER {
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

func (s *KernelSubsystem) Wait() error {
	db.DPrintf(db.KERNEL, "Wait subsystem for %v", s)
	defer db.DPrintf(db.KERNEL, "Wait subsystem done for %v", s)

	if !s.GetCrashed() {
		// Only wait if this proc has not been waited for already, since calling
		// WaitExit twice leads to an error.
		status, err := s.WaitExitKernelProc(s.p.GetPid(), s.how)
		if err != nil || !status.IsStatusEvicted() {
			db.DPrintf(db.ALWAYS, "shutdown subsystem [%v] exit with status %v err %v", s.p.GetPid(), status, err)
			return err
		}
	}

	if s.how == proc.HMSCHED || s.how == proc.HDOCKER {
		// Do nothing (already waited)
		return nil
	} else {
		db.DPrintf(db.KERNEL, "Wait subsystem via cmd %v", s)
		if err := s.cmd.Wait(); err != nil {
			return err
		}
	}
	if s.container != nil {
		db.DPrintf(db.KERNEL, "Container shutdown %v", s)
		return s.container.Shutdown()
	}
	return nil
}
