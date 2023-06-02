package kernel

import (
	"os/exec"
	"path"
	"syscall"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslib"
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
	how       procclnt.Thow
	cmd       *exec.Cmd
	container *container.Container
	crashed   bool
}

func makeSubsystemCmd(pclnt *procclnt.ProcClnt, k *Kernel, p *proc.Proc, how procclnt.Thow, cmd *exec.Cmd) *Subsystem {
	return &Subsystem{pclnt, k, p, how, cmd, nil, false}
}

func makeSubsystem(pclnt *procclnt.ProcClnt, k *Kernel, p *proc.Proc, how procclnt.Thow) *Subsystem {
	return makeSubsystemCmd(pclnt, k, p, how, nil)
}

func (k *Kernel) bootSubsystem(program string, args []string, how procclnt.Thow) (*Subsystem, error) {
	pid := proc.Tpid(program + "-" + proc.GenPid().String())
	p := proc.MakePrivProcPid(pid, program, args, true)
	ss := makeSubsystem(k.ProcClnt, k, p, how)
	return ss, ss.Run(k.namedAddr, how, k.Param.KernelId)
}

func (s *Subsystem) Run(namedAddr sp.Taddrs, how procclnt.Thow, kernelId string) error {
	if how == procclnt.HLINUX || how == procclnt.HSCHEDD {
		cmd, err := s.SpawnKernelProc(s.p, s.how, kernelId)
		if err != nil {
			return err
		}
		s.cmd = cmd
	} else {
		realm := sp.Trealm(s.p.Args[0])
		ptype := proc.ParseTtype(s.p.Args[1])
		if err := s.MkProc(s.p, procclnt.HDOCKER, kernelId); err != nil {
			return err
		}
		// XXX don't hard code
		h := sp.SIGMAHOME
		s.p.AppendEnv("PATH", h+"/bin/user:"+h+"/bin/user/common:"+h+"/bin/kernel:/usr/sbin:/usr/bin:/bin")
		s.p.Finalize("")
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
	err := s.WaitStart(s.p.GetPid())
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
	return GetSubsystemInfo(fsl, sp.KPIDS, ss.p.GetPid().String()).Ip
}

// Send SIGTERM to a system.
func (s *Subsystem) Terminate() error {
	db.DPrintf(db.KERNEL, "Terminate %v %v\n", s.cmd.Process.Pid, s.cmd)
	if s.how != procclnt.HLINUX {
		db.DFatalf("Tried to terminate a kernel subsystem spawned through procd: %v", s.p)
	}
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGTERM)
}

// Kill a subsystem, either by sending SIGKILL or Evicting it.
func (s *Subsystem) Kill() error {
	s.crashed = true
	if s.how == procclnt.HSCHEDD || s.how == procclnt.HDOCKER {
		db.DPrintf(db.ALWAYS, "Killing a kernel subsystem spawned through %v: %v", s.p, s.how)
		err := s.Evict(s.p.GetPid())
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error killing procd-spawned kernel proc: %v err %v", s.p.GetPid(), err)
		}
		return err
	}
	db.DPrintf(db.ALWAYS, "kill %v %v", s.cmd.Process.Pid, s.p.GetPid())
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGKILL)
}

func (s *Subsystem) Wait() {
	if s.how == procclnt.HSCHEDD || s.how == procclnt.HDOCKER {
		status, err := s.WaitExit(s.p.GetPid())
		if err != nil || !status.IsStatusOK() {
			db.DPrintf(db.ALWAYS, "Subsystem exit with status %v err %v", status, err)
		}
	} else {
		s.cmd.Wait()
	}
	if s.container != nil {
		s.container.Shutdown()
	}
}

type SubsystemInfo struct {
	Kpid proc.Tpid
	Ip   string
}

func MakeSubsystemInfo(kpid proc.Tpid, ip string) *SubsystemInfo {
	return &SubsystemInfo{kpid, ip}
}

func RegisterSubsystemInfo(fsl *fslib.FsLib, si *SubsystemInfo) {
	if err := fsl.PutFileJson(path.Join(proc.PROCDIR, SUBSYSTEM_INFO), 0777, si); err != nil {
		db.DFatalf("PutFileJson (%v): %v", path.Join(proc.PROCDIR, SUBSYSTEM_INFO), err)
	}
}

func GetSubsystemInfo(fsl *fslib.FsLib, kpids string, pid string) *SubsystemInfo {
	si := &SubsystemInfo{}
	if err := fsl.GetFileJson(path.Join(kpids, pid, SUBSYSTEM_INFO), si); err != nil {
		db.DFatalf("Error GetFileJson in subsystem info: %v", err)
		return nil
	}
	return si
}
