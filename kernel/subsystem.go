package kernel

import (
	"os/exec"
	"path"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type Subsystem struct {
	*procclnt.ProcClnt
	p        *proc.Proc
	viaProcd bool
	cmd      *exec.Cmd
}

func makeSubsystem(pclnt *procclnt.ProcClnt, p *proc.Proc, viaProcd bool) *Subsystem {
	return makeSubsystemCmd(pclnt, p, viaProcd, nil)
}

func makeSubsystemCmd(pclnt *procclnt.ProcClnt, p *proc.Proc, viaProcd bool, cmd *exec.Cmd) *Subsystem {
	return &Subsystem{pclnt, p, viaProcd, cmd}
}

func (s *Subsystem) Run(namedAddr []string) error {
	cmd, err := s.SpawnKernelProc(s.p, namedAddr, s.viaProcd)
	if err != nil {
		return err
	}
	s.cmd = cmd
	return s.WaitStart(s.p.Pid)
}

// Send SIGTERM to a system.
func (s *Subsystem) Terminate() error {
	if s.viaProcd {
		db.DFatalf("Tried to terminate a kernel subsystem spawned through procd: %v", s.p)
	}
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGTERM)
}

// Kill a subsystem, either by sending SIGKILL or Evicting it.
func (s *Subsystem) Kill() error {
	if s.viaProcd {
		db.DPrintf(db.ALWAYS, "Killing a kernel subsystem spawned through procd: %v", s.p)
		err := s.Evict(s.p.Pid)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error killing procd-spawned kernel proc: %v err %v", s.p.Pid, err)
		}
		return err
	}
	db.DPrintf(db.ALWAYS, "kill %v %v", s.cmd.Process.Pid, s.p.Pid)
	return syscall.Kill(s.cmd.Process.Pid, syscall.SIGKILL)
}

func (s *Subsystem) Wait() {
	if s.viaProcd {
		status, err := s.WaitExit(s.p.Pid)
		if err != nil || !status.IsStatusOK() {
			db.DPrintf(db.ALWAYS, "Subsystem exit with status %v err %v", status, err)
		}
	} else {
		s.cmd.Wait()
	}
}

type SubsystemInfo struct {
	Kpid    proc.Tpid
	Ip      string
	NodedId string
}

func MakeSubsystemInfo(kpid proc.Tpid, ip string, nodedId string) *SubsystemInfo {
	return &SubsystemInfo{kpid, ip, nodedId}
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
