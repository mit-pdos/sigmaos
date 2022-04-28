package kernel

import (
	"os/exec"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

type Subsystem struct {
	*procclnt.ProcClnt
	p   *proc.Proc
	cmd *exec.Cmd
}

func makeSubsystem(pclnt *procclnt.ProcClnt, p *proc.Proc) *Subsystem {
	return &Subsystem{pclnt, p, nil}
}

func (s *Subsystem) Run(bindir string, namedAddr []string) error {
	cmd, err := s.SpawnKernelProc(s.p, bindir, namedAddr)
	if err != nil {
		return err
	}
	s.cmd = cmd
	return s.WaitStart(s.p.Pid)
}

func (s *Subsystem) Monitor() {

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
		db.DFatalf("PutFileJson: %v", err)
	}
}

func GetSubsystemInfo(fsl *fslib.FsLib, kpid proc.Tpid) *SubsystemInfo {
	si := &SubsystemInfo{}
	if err := fsl.GetFileJson(path.Join(proc.KPIDS, kpid.String(), SUBSYSTEM_INFO), si); err != nil {
		db.DFatalf("GetFileJson: %v", err)
	}
	return si
}
